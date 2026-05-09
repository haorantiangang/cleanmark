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

type DouyinAdapter struct{}

func (d *DouyinAdapter) Name() string {
	return "抖音"
}

func (d *DouyinAdapter) SupportedDomains() []string {
	return []string{"douyin.com", "iesdouyin.com", "v.douyin.com"}
}

func (d *DouyinAdapter) Parse(ctx context.Context, url string) (*VideoInfo, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "/xg/video/") {
		xiguaAdapter := NewXiguaAdapter()
		return xiguaAdapter.Parse(ctx, finalURL)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	body := string(bodyBytes)

	renderDataRegex := regexp.MustCompile(`<script id="RENDER_DATA" type="application/json">(.*?)</script>`)
	matches := renderDataRegex.FindStringSubmatch(body)

	if len(matches) >= 2 {
		encodedData := matches[1]
		decodedData := strings.ReplaceAll(encodedData, "\\u002F", "/")
		decodedData = strings.ReplaceAll(decodedData, "\\u0026", "&")
		decodedData = strings.ReplaceAll(decodedData, "\\u003D", "=")
		decodedData = strings.ReplaceAll(decodedData, "\\u003F", "?")

		result := gjson.Parse(decodedData)

		videoData := result.Get("aweme.detail")
		if !videoData.Exists() {
			keys := result.Map()
			for _, v := range keys {
				if v.Get("aweme.detail").Exists() {
					videoData = v.Get("aweme.detail")
					break
				}
				if v.IsArray() {
					for _, item := range v.Array() {
						if item.Get("aweme.detail").Exists() {
							videoData = item.Get("aweme.detail")
							break
						}
					}
				}
			}
		}

		if videoData.Exists() {
			return d.extractVideoInfo(videoData, url)
		}
	}

	ssrDataRegex := regexp.MustCompile(`<script>window\._ROUTER_DATA\s*=\s*(.*?)</script>`)
	ssrMatches := ssrDataRegex.FindStringSubmatch(body)
	if len(ssrMatches) >= 2 {
		result := gjson.Parse(ssrMatches[1])

		paths := []string{
			"loaderData.video.detail",
			"video.detail",
			"loaderData.(video|aweme).detail",
			"aweme.detail",
		}

		for _, path := range paths {
			videoData := result.Get(path)
			if videoData.Exists() {
				return d.extractVideoInfo(videoData, url)
			}
		}

		result.ForEach(func(key, value gjson.Result) bool {
			if value.Get("detail").Exists() {
				videoData := value.Get("detail")
				if videoData.Get("desc").Exists() || videoData.Get("video").Exists() {
					_, err := d.extractVideoInfo(videoData, url)
					if err == nil {
						return false
					}
				}
			}
			return true
		})
	}

	initialStateRegex := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*(.*?);?\s*</script>`)
	initialMatches := initialStateRegex.FindStringSubmatch(body)
	if len(initialMatches) >= 2 {
		data := strings.TrimSuffix(initialMatches[1], ";")
		result := gjson.Parse(data)

		videoData := result.Get("aweme.detail")
		if videoData.Exists() {
			return d.extractVideoInfo(videoData, url)
		}
	}

	videoIdRegex := regexp.MustCompile(`/video/(\d+)`)
	videoIdMatches := videoIdRegex.FindStringSubmatch(resp.Request.URL.String())
	if len(videoIdMatches) >= 2 {
		videoID := videoIdMatches[1]
		return d.parseByVideoID(ctx, videoID, url)
	}

	pageUrl := resp.Request.URL.String()
	if strings.Contains(pageUrl, "/xg/video/") {
		xgVideoRegex := regexp.MustCompile(`/xg/video/(\d+)`)
		xgMatches := xgVideoRegex.FindStringSubmatch(pageUrl)
		if len(xgMatches) >= 2 {
			videoID := xgMatches[1]
			return d.parseByVideoID(ctx, videoID, url)
		}
	}

	if strings.Contains(body, "aweme") {
		awemeIdRegex := regexp.MustCompile(`"aweme_id":"([^"]+)"`)
		awemeMatches := awemeIdRegex.FindStringSubmatch(body)
		if len(awemeMatches) >= 2 {
			videoID := awemeMatches[1]
			return d.parseByVideoID(ctx, videoID, url)
		}
	}

	return nil, fmt.Errorf("未找到视频数据，抖音页面结构可能已更新，请稍后重试或联系管理员")
}

func (d *DouyinAdapter) extractVideoInfo(videoData gjson.Result, originalURL string) (*VideoInfo, error) {
	title := videoData.Get("desc").String()
	if title == "" {
		title = videoData.Get("title").String()
	}
	if title == "" {
		title = "抖音视频"
	}

	playAddr := videoData.Get("video.play_addr.url_list.0").String()
	if playAddr == "" {
		playAddr = videoData.Get("video.bit_rate.0.play_addr.url_list.0").String()
	}
	if playAddr == "" {
		playAddr = videoData.Get("video.play_addr_h264.url_list.0").String()
	}
	if playAddr == "" {
		playAddr = videoData.Get("video.download_addr.url_list.0").String()
	}

	cover := videoData.Get("video.cover.url_list.0").String()
	if cover == "" {
		cover = videoData.Get("video.dynamic_cover.url_list.0").String()
	}
	if cover == "" {
		cover = videoData.Get("video.origin_cover.url_list.0").String()
	}

	duration := int(videoData.Get("video.duration").Int()) / 1000
	author := videoData.Get("author.nickname").String()
	if author == "" {
		author = videoData.Get("author.unique_id").String()
	}
	if author == "" {
		author = videoData.Get("author.name").String()
	}

	return &VideoInfo{
		Title:       title,
		CoverURL:    cover,
		VideoURL:    playAddr,
		OriginalURL: originalURL,
		Duration:    duration,
		Author:      author,
		Platform:    "douyin",
	}, nil
}

func (d *DouyinAdapter) parseByVideoID(ctx context.Context, videoID string, originalURL string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/?item_ids=%s", videoID)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := gjson.Parse(string(body))

	itemList := result.Get("item_list.0")
	if !itemList.Exists() {
		return nil, fmt.Errorf("API返回数据为空")
	}

	title := itemList.Get("desc").String()
	playAddr := itemList.Get("video.play_addr.url_list.0").String()
	cover := itemList.Get("video.cover.url_list.0").String()
	duration := int(itemList.Get("video.duration").Int()) / 1000
	author := itemList.Get("author.nickname").String()

	return &VideoInfo{
		Title:       title,
		CoverURL:    cover,
		VideoURL:    playAddr,
		OriginalURL: originalURL,
		Duration:    duration,
		Author:      author,
		Platform:    "douyin",
	}, nil
}
