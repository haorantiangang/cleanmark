package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type TikTokAdapter struct{}

func (t *TikTokAdapter) Name() string {
	return "TikTok"
}

func (t *TikTokAdapter) SupportedDomains() []string {
	return []string{"tiktok.com", "vm.tiktok.com"}
}

func (t *TikTokAdapter) Parse(ctx context.Context, inputURL string) (*VideoInfo, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", inputURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	body := string(bodyBytes)

	scriptRegex := regexp.MustCompile(`<script id="SIGI_STATE" type="application\/json">(.*?)</script>`)
	matches := scriptRegex.FindStringSubmatch(body)

	if len(matches) < 2 {
		hydratedRegex := regexp.MustCompile(`<script id="__NEXT_DATA__" type="application\/json">(.*?)</script>`)
		matches = hydratedRegex.FindStringSubmatch(body)
	}

	if len(matches) < 2 {
		metaOGRegex := regexp.MustCompile(`<meta property="og:video:url" content="(.*?)"`)
		ogMatches := metaOGRegex.FindStringSubmatch(body)
		
		if len(ogMatches) >= 2 {
			titleRegex := regexp.MustCompile(`<meta property="og:title" content="(.*?)"`)
			titleMatches := titleRegex.FindStringSubmatch(body)
			
			title := "TikTok Video"
			if len(titleMatches) >= 2 {
				title = titleMatches[1]
			}
			
			return &VideoInfo{
				Title:       title,
				VideoURL:    ogMatches[1],
				OriginalURL: finalURL,
				Platform:    "tiktok",
			}, nil
		}
		
		return nil, fmt.Errorf("无法解析TikTok页面，请检查链接是否正确")
	}

	jsonStr := matches[1]
	
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("解析JSON数据失败: %w", err)
	}

	videoItem := t.extractVideoItem(data)
	if videoItem == nil {
		return nil, fmt.Errorf("未找到视频数据")
	}

	title := t.getString(videoItem, "desc")
	if title == "" {
		title = "TikTok Video"
	}

	authorInfo := t.getMap(videoItem, "author")
	author := ""
	if authorInfo != nil {
		author = t.getString(authorInfo, "nickname")
		if author == "" {
			author = t.getString(authorInfo, "uniqueId")
		}
	}

	video := t.getMap(videoItem, "video")
	if video == nil {
		return nil, fmt.Errorf("未找到视频信息")
	}

	playAddr := t.getMap(video, "playAddr")
	var videoURL string
	if playAddr != nil {
		urlList := t.getArray(playAddr, "urlList")
		if len(urlList) > 0 {
			videoURL = urlList[0]
		}
	}

	if videoURL == "" {
		downloadAddr := t.getMap(video, "downloadAddr")
		if downloadAddr != nil {
			urlList := t.getArray(downloadAddr, "urlList")
			if len(urlList) > 0 {
				videoURL = urlList[0]
			}
		}
	}

	if videoURL == "" {
		videoURL = t.getString(video, "playApi")
	}

	cover := ""
	originCover := t.getMap(video, "originCover")
	if originCover != nil {
		urlList := t.getArray(originCover, "urlList")
		if len(urlList) > 0 {
			cover = urlList[0]
		}
	}
	if cover == "" {
		cover = t.getString(video, "cover")
	}

	duration := 0
	if d, ok := video["duration"].(float64); ok {
		duration = int(d)
	}

	if videoURL == "" {
		return nil, fmt.Errorf("无法获取视频下载地址")
	}

	parsedURL, err := url.Parse(videoURL)
	if err == nil && parsedURL.Host != "" {
		q := parsedURL.Query()
		q.Set("rm", "false")
		parsedURL.RawQuery = q.Encode()
		videoURL = parsedURL.String()
	}

	info := &VideoInfo{
		Title:       title,
		CoverURL:    cover,
		VideoURL:    videoURL,
		OriginalURL: finalURL,
		Duration:    duration,
		Author:      author,
		Platform:    "tiktok",
	}

	return info, nil
}

func (t *TikTokAdapter) extractVideoItem(data map[string]interface{}) map[string]interface{} {
	if itemModuleList, ok := data["ItemModule"].([]interface{}); ok && len(itemModuleList) > 0 {
		if item, ok := itemModuleList[0].(map[string]interface{}); ok {
			return item
		}
	}

	if items, ok := data["items"].([]interface{}); ok {
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if _, exists := itemMap["video"]; exists {
					return itemMap
				}
			}
		}
	}

	if videoData, ok := data["VideoModule"].(map[string]interface{}); ok {
		return videoData
	}

	for key, value := range data {
		if key == "ItemModule" || key == "VideoModule" {
			continue
		}
		if m, ok := value.(map[string]interface{}); ok {
			if item := t.extractVideoItemFromMap(m); item != nil {
				return item
			}
		}
	}

	return nil
}

func (t *TikTokAdapter) extractVideoItemFromMap(m map[string]interface{}) map[string]interface{} {
	if itemModuleList, ok := m["ItemModule"].([]interface{}); ok && len(itemModuleList) > 0 {
		if item, ok := itemModuleList[0].(map[string]interface{}); ok {
			return item
		}
	}
	return nil
}

func (t *TikTokAdapter) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func (t *TikTokAdapter) getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

func (t *TikTokAdapter) getArray(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return []string{}
}
