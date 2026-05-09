package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type BilibiliAdapter struct{}

func (b *BilibiliAdapter) Name() string {
	return "B站"
}

func (b *BilibiliAdapter) SupportedDomains() []string {
	return []string{"bilibili.com", "b23.tv", "m.bilibili.com"}
}

func (b *BilibiliAdapter) Parse(ctx context.Context, url string) (*VideoInfo, error) {
	finalURL := url

	if strings.Contains(url, "b23.tv") {
		realURL, err := b.resolveShortURL(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("解析短链接失败: %w", err)
		}
		finalURL = realURL
	}

	bvID := b.extractBV(finalURL)
	if bvID == "" {
		return nil, fmt.Errorf("无法提取BV号，请检查链接格式")
	}

	apiURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvID)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建API请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API请求失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	result := gjson.ParseBytes(bodyBytes)

	code := result.Get("code").Int()
	if code != 0 {
		msg := result.Get("message").String()
		return nil, fmt.Errorf("B站API错误(%d): %s", code, msg)
	}

	data := result.Get("data")
	if !data.Exists() {
		return nil, fmt.Errorf("获取视频数据失败")
	}

	title := data.Get("title").String()
	cover := data.Get("pic").String()
	author := data.Get("owner.name").String()
	duration := int(data.Get("duration").Int())
	bvid := data.Get("bvid").String()
	cid := data.Get("cid").Int()

	videoURL, err := b.getVideoStreamURL(ctx, bvid, cid)
	if err != nil {
		return nil, fmt.Errorf("获取视频流失败: %w", err)
	}

	info := &VideoInfo{
		Title:       title,
		CoverURL:    cover,
		VideoURL:    videoURL,
		OriginalURL: url,
		Duration:    duration,
		Author:      author,
		Platform:    "bilibili",
	}

	return info, nil
}

func (b *BilibiliAdapter) extractBV(url string) string {
	re := regexp.MustCompile(`BV[a-zA-Z0-9]+`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func (b *BilibiliAdapter) getVideoStreamURL(ctx context.Context, bvid string, cid int64) (string, error) {
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?bvid=%s&cid=%d&qn=80&fnval=16", bvid, cid)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	result := gjson.ParseBytes(bodyBytes)

	code := result.Get("code").Int()
	if code != 0 {
		return "", fmt.Errorf("B站API错误: %s", result.Get("message").String())
	}

	data := result.Get("data")
	if !data.Exists() {
		return "", fmt.Errorf("获取播放数据失败")
	}

	dash := data.Get("dash")
	if dash.Exists() {
		video := dash.Get("video")
		if video.Exists() && video.IsArray() && len(video.Array()) > 0 {
			firstVideo := video.Array()[0]
			baseURL := firstVideo.Get("baseUrl").String()
			if baseURL != "" {
				return baseURL, nil
			}
			backupURL := firstVideo.Get("backupUrl")
			if backupURL.Exists() && backupURL.IsArray() && len(backupURL.Array()) > 0 {
				return backupURL.Array()[0].String(), nil
			}
		}
	}

	durl := data.Get("durl")
	if durl.Exists() && durl.IsArray() && len(durl.Array()) > 0 {
		firstDurl := durl.Array()[0]
		url := firstDurl.Get("url").String()
		if url != "" {
			return url, nil
		}
	}

	return "", fmt.Errorf("未找到视频流地址")
}

func (b *BilibiliAdapter) buildPlayerURL(bvid string, cid int64) string {
	return fmt.Sprintf("https://www.bilibili.com/video/%s?p=1&cid=%d", bvid, cid)
}

func (b *BilibiliAdapter) resolveShortURL(ctx context.Context, shortURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", shortURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	
	if !strings.Contains(finalURL, "bilibili.com") && !strings.Contains(finalURL, "b23.tv") {
		return "", fmt.Errorf("无效的重定向地址")
	}

	return finalURL, nil
}

func parseDuration(durationStr string) int {
	parts := strings.Split(durationStr, ":")
	if len(parts) == 2 {
		minutes, _ := strconv.Atoi(parts[0])
		seconds, _ := strconv.Atoi(parts[1])
		return minutes*60 + seconds
	} else if len(parts) == 3 {
		hours, _ := strconv.Atoi(parts[0])
		minutes, _ := strconv.Atoi(parts[1])
		seconds, _ := strconv.Atoi(parts[2])
		return hours*3600 + minutes*60 + seconds
	}
	return 0
}
