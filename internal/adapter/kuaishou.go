package adapter

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/tidwall/gjson"
)

type KuaishouAdapter struct{}

func (k *KuaishouAdapter) Name() string {
	return "快手"
}

func (k *KuaishouAdapter) SupportedDomains() []string {
	return []string{"kuaishou.com", "gifshow.com", "v.kuaishou.com"}
}

func (k *KuaishouAdapter) Parse(ctx context.Context, url string) (*VideoInfo, error) {
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
	req.Header.Set("Referer", "https://www.kuaishou.com/")

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

	log.Printf("[快手] 请求URL: %s, 响应状态: %d, 响应长度: %d", url, resp.StatusCode, len(bodyBytes))

	if info := k.extractFromInlineJSON(body, url); info != nil {
		log.Printf("[快手] extractFromInlineJSON 成功: title=%s, videoURL=%s", info.Title, info.VideoURL)
		return info, nil
	} else {
		log.Printf("[快手] extractFromInlineJSON 失败")
	}

	scriptDataRegex := regexp.MustCompile(`<script[^>]*>\s*(\{[^<]*"photo"[^<]*\})\s*</script>`)
	scriptMatches := scriptDataRegex.FindStringSubmatch(body)
	if len(scriptMatches) >= 2 {
		result := gjson.Parse(scriptMatches[1])
		if info := k.extractFromScriptData(result, url); info != nil {
			return info, nil
		}
	}

	pageDataRegex := regexp.MustCompile(`window\.pageData\s*=\s*(\{.*?\})\s*;?\s*</script>`)
	matches := pageDataRegex.FindStringSubmatch(body)

	if len(matches) >= 2 {
		result := gjson.Parse(matches[1])
		if info := k.extractFromPageData(result, url); info != nil {
			return info, nil
		}
	}

	initialStateRegex := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*(\{.*?\})\s*;?\s*</script>`)
	matches = initialStateRegex.FindStringSubmatch(body)

	if len(matches) >= 2 {
		result := gjson.Parse(matches[1])
		if info := k.extractFromInitialState(result, url); info != nil {
			return info, nil
		}
	}

	ssrDataRegex := regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)
	ssrMatches := ssrDataRegex.FindStringSubmatch(body)
	if len(ssrMatches) >= 2 {
		result := gjson.Parse(ssrMatches[1])
		if info := k.extractFromNextData(result, url); info != nil {
			return info, nil
		}
	}

	metaRegex := regexp.MustCompile(`<meta property="og:video:url" content="(.*?)"`)
	metaMatches := metaRegex.FindStringSubmatch(body)
	if len(metaMatches) >= 2 {
		title := "快手视频"
		titleRegex := regexp.MustCompile(`<meta property="og:title" content="(.*?)"`)
		if titleMatches := titleRegex.FindStringSubmatch(body); len(titleMatches) >= 2 {
			title = titleMatches[1]
		}

		coverRegex := regexp.MustCompile(`<meta property="og:image" content="(.*?)"`)
		cover := ""
		if coverMatches := coverRegex.FindStringSubmatch(body); len(coverMatches) >= 2 {
			cover = coverMatches[1]
		}

		return &VideoInfo{
			Title:       title,
			CoverURL:    cover,
			VideoURL:    metaMatches[1],
			OriginalURL: url,
			Platform:    "kuaishou",
		}, nil
	}

	videoIdRegex := regexp.MustCompile(`/photo/([^/\?]+)`)
	if videoIdMatches := videoIdRegex.FindStringSubmatch(resp.Request.URL.String()); len(videoIdMatches) >= 2 {
		videoID := videoIdMatches[1]
		if info, err := k.parseByVideoID(ctx, videoID, url); err == nil {
			return info, nil
		}
	}

	shortIdRegex := regexp.MustCompile(`v\.kuaishou\.com/([A-Za-z0-9]+)`)
	if shortIdMatches := shortIdRegex.FindStringSubmatch(url); len(shortIdMatches) >= 2 {
		if info, err := k.parseByShortID(ctx, shortIdMatches[1], url); err == nil {
			return info, nil
		}
	}

	return nil, fmt.Errorf("未找到视频数据，快手页面结构可能已更新，请稍后重试")
}

func (k *KuaishouAdapter) extractFromInlineJSON(body string, originalURL string) *VideoInfo {
	titleRegex := regexp.MustCompile(`"caption":"([^"]*)"`)
	title := ""
	if matches := titleRegex.FindStringSubmatch(body); len(matches) >= 2 {
		title = matches[1]
	}

	if title == "" {
		return nil
	}

	var videoURL string
	videoURLRegex := regexp.MustCompile(`"mainMvUrls":\[\s*\{[^}]*"url":"([^"]*)"`)
	if matches := videoURLRegex.FindStringSubmatch(body); len(matches) >= 2 {
		videoURL = matches[1]
	}

	if videoURL == "" {
		backupURLRegex := regexp.MustCompile(`"url":"(https://[^"]*\.mp4[^"]*)"`)
		if matches := backupURLRegex.FindStringSubmatch(body); len(matches) >= 2 {
			videoURL = matches[1]
		}
	}

	var coverURL string
	coverRegex := regexp.MustCompile(`"coverUrls":\[\s*\{[^}]*"url":"([^"]*)"`)
	if matches := coverRegex.FindStringSubmatch(body); len(matches) >= 2 {
		coverURL = matches[1]
	}

	var author string
	authorRegex := regexp.MustCompile(`"userName":"([^"]*)"`)
	if matches := authorRegex.FindStringSubmatch(body); len(matches) >= 2 {
		author = matches[1]
	}

	var duration int
	durationRegex := regexp.MustCompile(`"duration":(\d+)`)
	if matches := durationRegex.FindStringSubmatch(body); len(matches) >= 2 {
		duration = parseInt(matches[1]) / 1000
	}

	if videoURL == "" {
		return nil
	}

	return &VideoInfo{
		Title:       title,
		CoverURL:    coverURL,
		VideoURL:    videoURL,
		OriginalURL: originalURL,
		Duration:    duration,
		Author:      author,
		Platform:    "kuaishou",
	}
}

func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

func (k *KuaishouAdapter) extractFromScriptData(result gjson.Result, originalURL string) *VideoInfo {
	photoData := result.Get("photo")
	if !photoData.Exists() {
		return nil
	}

	title := photoData.Get("caption").String()
	if title == "" {
		title = "快手视频"
	}

	var videoURL string
	mainMvUrls := photoData.Get("mainMvUrls")
	if mainMvUrls.Exists() && mainMvUrls.IsArray() {
		firstUrl := mainMvUrls.Array()[0]
		videoURL = firstUrl.Get("url").String()
	}

	if videoURL == "" {
		manifest := photoData.Get("manifest")
		if manifest.Exists() {
			adaptationSet := manifest.Get("adaptationSet")
			if adaptationSet.Exists() && adaptationSet.IsArray() {
				firstAdaptation := adaptationSet.Array()[0]
				representation := firstAdaptation.Get("representation")
				if representation.Exists() && representation.IsArray() {
					for _, rep := range representation.Array() {
						url := rep.Get("url").String()
						if url != "" {
							videoURL = url
							break
						}
					}
				}
			}
		}
	}

	var coverURL string
	coverUrls := photoData.Get("coverUrls")
	if coverUrls.Exists() && coverUrls.IsArray() {
		firstCover := coverUrls.Array()[0]
		coverURL = firstCover.Get("url").String()
	}

	author := photoData.Get("userName").String()

	duration := int(photoData.Get("duration").Int()) / 1000

	return &VideoInfo{
		Title:       title,
		CoverURL:    coverURL,
		VideoURL:    videoURL,
		OriginalURL: originalURL,
		Duration:    duration,
		Author:      author,
		Platform:    "kuaishou",
	}
}

func (k *KuaishouAdapter) extractFromPageData(result gjson.Result, originalURL string) *VideoInfo {
	var videoData gjson.Result

	if result.Get("photoData").Exists() {
		videoData = result.Get("photoData")
	} else if result.Get("video").Exists() {
		videoData = result.Get("video")
	} else if result.Get("detail").Exists() {
		videoData = result.Get("detail")
	} else if result.Get("photo").Exists() {
		videoData = result.Get("photo")
	}

	if !videoData.Exists() {
		return nil
	}

	return k.extractVideoInfo(videoData, originalURL)
}

func (k *KuaishouAdapter) extractFromInitialState(result gjson.Result, originalURL string) *VideoInfo {
	paths := []string{
		"photo.photo",
		"video.photo",
		"detail.photo",
		"photo",
	}

	for _, path := range paths {
		if videoData := result.Get(path); videoData.Exists() {
			if info := k.extractVideoInfo(videoData, originalURL); info != nil {
				return info
			}
		}
	}

	return nil
}

func (k *KuaishouAdapter) extractFromNextData(result gjson.Result, originalURL string) *VideoInfo {
	paths := []string{
		"props.pageProps.photo",
		"props.initialProps.photo",
		"query.photo",
	}

	for _, path := range paths {
		if videoData := result.Get(path); videoData.Exists() {
			if info := k.extractVideoInfo(videoData, originalURL); info != nil {
				return info
			}
		}
	}

	return nil
}

func (k *KuaishouAdapter) extractVideoInfo(videoData gjson.Result, originalURL string) *VideoInfo {
	title := videoData.Get("caption").String()
	if title == "" {
		title = videoData.Get("photoTitle").String()
	}
	if title == "" {
		title = videoData.Get("name").String()
	}
	if title == "" {
		title = "快手视频"
	}

	var videoURL string

	if photo := videoData.Get("photo"); photo.Exists() {
		videoURL = photo.Get("mainMvUrl").String()
		if videoURL == "" {
			videoURL = photo.Get("url").String()
		}
		if videoURL == "" {
			videoURL = photo.Get("playUrl").String()
		}
	}

	if videoURL == "" {
		videoURL = videoData.Get("playUrl").String()
	}
	if videoURL == "" {
		videoURL = videoData.Get("mainMvUrl").String()
	}
	if videoURL == "" {
		videoURL = videoData.Get("srcNoMark").String()
	}
	if videoURL == "" {
		videoURL = videoData.Get("videoUrl").String()
	}

	cover := videoData.Get("coverUrl").String()
	if cover == "" {
		cover = videoData.Get("headUrl").String()
	}
	if cover == "" {
		cover = videoData.Get("thumbnail").String()
	}

	author := videoData.Get("userName").String()
	if author == "" {
		author = videoData.Get("authorName").String()
	}
	if author == "" {
		author = videoData.Get("user.name").String()
	}

	duration := int(videoData.Get("duration").Int())
	if duration == 0 {
		duration = int(videoData.Get("photo.duration").Int())
	}

	return &VideoInfo{
		Title:       title,
		CoverURL:    cover,
		VideoURL:    videoURL,
		OriginalURL: originalURL,
		Duration:    duration / 1000,
		Author:      author,
		Platform:    "kuaishou",
	}
}

func (k *KuaishouAdapter) parseByVideoID(ctx context.Context, videoID string, originalURL string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.kuaishou.com/short-video/%s", videoID)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	pageDataRegex := regexp.MustCompile(`window\.pageData\s*=\s*(\{.*?\})\s*;?\s*</script>`)
	matches := pageDataRegex.FindStringSubmatch(string(body))

	if len(matches) >= 2 {
		result := gjson.Parse(matches[1])
		if info := k.extractFromPageData(result, originalURL); info != nil {
			return info, nil
		}
	}

	return nil, fmt.Errorf("通过视频ID解析失败")
}

func (k *KuaishouAdapter) parseByShortID(ctx context.Context, shortID string, originalURL string) (*VideoInfo, error) {
	resolveURL := fmt.Sprintf("https://v.kuaishou.com/%s", shortID)

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", resolveURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	videoIdRegex := regexp.MustCompile(`/photo/([^/\?]+)`)
	if matches := videoIdRegex.FindStringSubmatch(finalURL); len(matches) >= 2 {
		return k.parseByVideoID(ctx, matches[1], originalURL)
	}

	return nil, fmt.Errorf("无法解析短链接")
}
