package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type XiaohongshuAdapter struct{}

func (x *XiaohongshuAdapter) Name() string {
	return "小红书"
}

func (x *XiaohongshuAdapter) SupportedDomains() []string {
	return []string{"xiaohongshu.com", "xhslink.com", "xhslink.com"}
}

func (x *XiaohongshuAdapter) Parse(ctx context.Context, url string) (*VideoInfo, error) {
	finalURL := url

	log.Printf("[小红书] 原始URL: %s", url)

	if strings.Contains(url, "xhslink.com") {
		realURL, err := resolveRedirect(ctx, url)
		if err != nil {
			log.Printf("[小红书] 解析短链接失败: %v", err)
			return nil, fmt.Errorf("解析短链接失败: %w", err)
		}
		finalURL = realURL
		log.Printf("[小红书] 短链接重定向到: %s", finalURL)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	body := string(bodyBytes)
	log.Printf("[小红书] 响应状态: %d, 响应长度: %d", resp.StatusCode, len(bodyBytes))

	initialStateRegex := regexp.MustCompile(`window.__INITIAL_STATE__\s*=\s*(\{.+?\})\s*</script>`)
	matches := initialStateRegex.FindStringSubmatch(body)

	if len(matches) < 2 {
		log.Printf("[小红书] 未找到 __INITIAL_STATE__ 数据")
		return nil, fmt.Errorf("未找到页面数据，请检查链接是否正确")
	}

	jsonStr := matches[1]

	jsonStr = strings.ReplaceAll(jsonStr, "undefined", "null")

	result := gjson.Parse(jsonStr)

	noteData := result.Get("noteNoteDetailMap")
	if !noteData.Exists() {
		log.Printf("[小红书] noteNoteDetailMap 不存在，尝试 noteDetailMap")
		noteData = result.Get("noteDetailMap")
	}

	if !noteData.Exists() {
		log.Printf("[小红书] noteDetailMap 不存在，尝试 note")
		noteData = result.Get("note")
		if noteData.Exists() {
			log.Printf("[小红书] 找到 note 键，noteData 类型: %s", noteData.Type.String())
			
			if noteData.Get("noteDetailMap").Exists() {
				log.Printf("[小红书] note.noteDetailMap 存在，使用它")
				noteData = noteData.Get("noteDetailMap")
			}
		}
	}

	if !noteData.Exists() || !noteData.IsObject() {
		log.Printf("[小红书] 未找到笔记数据，noteData.Exists: %v, noteData.IsObject: %v", noteData.Exists(), noteData.IsObject())
		
		keys := make([]string, 0)
		result.ForEach(func(key, value gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
		log.Printf("[小红书] 可用的顶级键: %v", keys)
		
		return nil, fmt.Errorf("解析笔记数据失败")
	}

	var noteDetail gjson.Result
	
	noteData.ForEach(func(key, value gjson.Result) bool {
		if value.Get("note").Exists() {
			noteDetail = value.Get("note")
			log.Printf("[小红书] 从 noteDetailMap[%s].note 获取数据", key.String())
			return false
		}
		return true
	})

	if !noteDetail.Exists() {
		log.Printf("[小红书] noteDetail 不存在，尝试直接使用 noteData")
		noteDetail = noteData
	}

	title := noteDetail.Get("title").String()
	if title == "" {
		title = noteDetail.Get("desc").String()
	}
	if len(title) > 100 {
		title = title[:100] + "..."
	}
	if title == "" {
		title = "小红书内容"
	}

	var videoURL string
	var coverURL string
	var noteType string = "image"

	if noteDetail.Get("video").Exists() {
		noteType = "video"
		videoData := noteDetail.Get("video")
		
		log.Printf("[小红书] video 字段存在，开始提取视频URL")
		
		videoData.ForEach(func(key, value gjson.Result) bool {
			log.Printf("[小红书] video 子键: %s, 类型: %s", key.String(), value.Type.String())
			return true
		})
		
		consumer := videoData.Get("consumer.originVideoKey")
		if consumer.Exists() {
			videoURL = consumer.String()
			log.Printf("[小红书] 从 consumer.originVideoKey 获取: %s", videoURL)
		}
		
		masterUrl := videoData.Get("media.stream.h264.0.masterUrl")
		if masterUrl.Exists() {
			videoURL = masterUrl.String()
			log.Printf("[小红书] 从 media.stream.h264.0.masterUrl 获取: %s", videoURL)
		}
		
		stream := videoData.Get("media.stream")
		if stream.Exists() {
			stream.ForEach(func(key, value gjson.Result) bool {
				log.Printf("[小红书] stream 子键: %s", key.String())
				if value.IsArray() && len(value.Array()) > 0 {
					first := value.Array()[0]
					if first.Get("masterUrl").Exists() {
						videoURL = first.Get("masterUrl").String()
						log.Printf("[小红书] 从 stream.%s[0].masterUrl 获取: %s", key.String(), videoURL)
						return false
					}
				}
				return true
			})
		}

		coverObj := noteDetail.Get("imageList.0.infoList.-1.url")
		if coverObj.Exists() {
			coverURL = coverObj.String()
		}
	}

	if noteType == "image" && noteDetail.Get("imageList").Exists() {
		firstImage := noteDetail.Get("imageList.0.infoList.-1.url")
		if firstImage.Exists() {
			videoURL = firstImage.String()
			coverURL = firstImage.String()
		}
	}

	if videoURL == "" && noteType == "video" {
		log.Printf("[小红书] 视频URL提取失败，尝试其他路径")
	}

	author := noteDetail.Get("user.nickname").String()

	if noteType == "image" {
		title = "[图文] " + title
	}

	log.Printf("[小红书] 笔记类型: %s, 标题: %s, 作者: %s, videoURL: %s", noteType, title, author, videoURL)

	info := &VideoInfo{
		Title:       title,
		CoverURL:    coverURL,
		VideoURL:    videoURL,
		OriginalURL: url,
		Author:      author,
		Platform:    "xiaohongshu",
	}

	resultJSON, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println("小红书解析结果:", string(resultJSON))

	return info, nil
}

func resolveRedirect(ctx context.Context, url string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return url, nil
}
