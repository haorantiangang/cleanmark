package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type XiguaAdapter struct{}

func NewXiguaAdapter() *XiguaAdapter {
	return &XiguaAdapter{}
}

func (x *XiguaAdapter) Name() string {
	return "西瓜视频"
}

func (x *XiguaAdapter) SupportedDomains() []string {
	return []string{"ixigua.com", "toutiao.com", "iesdouyin.com"}
}

func (x *XiguaAdapter) Parse(ctx context.Context, url string) (*VideoInfo, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

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

	scriptRegex := regexp.MustCompile(`<script type="application\/ld\+json">(\{.+?\})<\/script>`)
	matches := scriptRegex.FindStringSubmatch(body)

	if len(matches) < 2 {
		renderDataRegex := regexp.MustCompile(`window.__INITIAL_STATE__\s*=\s*(\{.+?\})\s*</script>`)
		matches = renderDataRegex.FindStringSubmatch(body)
	}

	if len(matches) < 2 {
		return nil, fmt.Errorf("未找到西瓜视频数据，请检查链接是否正确")
	}

	jsonStr := matches[1]

	if strings.Contains(jsonStr, "__INITIAL_STATE__") {
		result := gjson.Parse(jsonStr)
		
		videoData := result.Get("videoDetailData.videoInfo")
		if !videoData.Exists() {
			keys := result.Map()
			for _, v := range keys {
				if v.Get("videoDetailData.videoInfo").Exists() {
					videoData = v.Get("videoDetailData.videoInfo")
					break
				}
			}
		}

		if !videoData.Exists() {
			return nil, fmt.Errorf("解析视频详情失败")
		}

		title := videoData.Get("title").String()
		cover := videoData.Get("poster_url").String()
		if cover == "" {
			cover = videoData.Get("large_cover.url_list.0").String()
		}
		
		videoURL := videoData.Get("video_resource.normal.video_url").String()
		if videoURL == "" {
			videoURL = videoData.Get("video_resource.dynamic_video.url").String()
		}
		if videoURL == "" {
			videoURL = videoData.Get("play_api").String()
		}

		duration := int(videoData.Get("duration").Int())
		author := videoData.Get("user.name").String()

		return &VideoInfo{
			Title:       title,
			CoverURL:    cover,
			VideoURL:    videoURL,
			OriginalURL: url,
			Duration:    duration,
			Author:      author,
			Platform:    "xigua",
		}, nil
	}

	result := gjson.Parse(jsonStr)
	
	name := result.Get("name").String()
	contentURL := result.Get("embedUrl").String()
	thumbnailURL := result.Get("thumbnailUrl").String()
	
	if contentURL == "" {
		contentURL = result.Get("url").String()
	}

	return &VideoInfo{
		Title:       name,
		CoverURL:    thumbnailURL,
		VideoURL:    contentURL,
		OriginalURL: url,
		Platform:    "xigua",
	}, nil
}
