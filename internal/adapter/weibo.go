package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type WeiboAdapter struct{}

func NewWeiboAdapter() *WeiboAdapter {
	return &WeiboAdapter{}
}

func (w *WeiboAdapter) SupportedPlatforms() []string {
	return []string{"weibo"}
}

func (w *WeiboAdapter) SupportedDomains() []string {
	return []string{"weibo.com", "weibo.cn", "m.weibo.cn", "video.weibo.com"}
}

func (w *WeiboAdapter) Parse(ctx context.Context, url string) (*VideoInfo, error) {
	if strings.Contains(url, "video.weibo.com") {
		return w.parseVideoPage(ctx, url)
	}

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

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

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

	videoURLRegex := regexp.MustCompile(`"stream_url":\s*"([^"]+)"`)
	matches := videoURLRegex.FindStringSubmatch(body)

	var videoURL string
	if len(matches) >= 2 {
		videoURL = matches[1]
		videoURL = strings.ReplaceAll(videoURL, "\\/", "/")
	}

	if videoURL == "" {
		pageInfoRegex := regexp.MustCompile(`"page_info":\s*\{[^}]*"media_info":\s*\{[^}]*"stream_url":\s*"([^"]+)"`)
		matches = pageInfoRegex.FindStringSubmatch(body)
		if len(matches) >= 2 {
			videoURL = matches[1]
			videoURL = strings.ReplaceAll(videoURL, "\\/", "/")
		}
	}

	if videoURL == "" {
		videoSrcRegex := regexp.MustCompile(`video_src=['"]([^'"]+)['"]`)
		matches = videoSrcRegex.FindStringSubmatch(body)
		if len(matches) >= 2 {
			videoURL = matches[1]
		}
	}

	var title string
	titleRegex := regexp.MustCompile(`<title>([^<]+)</title>`)
	titleMatches := titleRegex.FindStringSubmatch(body)
	if len(titleMatches) >= 2 {
		title = titleMatches[1]
	}
	if title == "" {
		title = "微博视频"
	}

	var coverURL string
	coverRegex := regexp.MustCompile(`"cover_image":\s*"([^"]+)"`)
	coverMatches := coverRegex.FindStringSubmatch(body)
	if len(coverMatches) >= 2 {
		coverURL = coverMatches[1]
		coverURL = strings.ReplaceAll(coverURL, "\\/", "/")
		if !strings.HasPrefix(coverURL, "http") {
			coverURL = "https:" + coverURL
		}
	}

	var author string
	authorRegex := regexp.MustCompile(`"author":\s*"([^"]+)"`)
	authorMatches := authorRegex.FindStringSubmatch(body)
	if len(authorMatches) >= 2 {
		author = authorMatches[1]
	}

	if videoURL == "" {
		return nil, fmt.Errorf("未找到视频播放地址")
	}

	if !strings.HasPrefix(videoURL, "http") {
		videoURL = "https:" + videoURL
	}

	info := &VideoInfo{
		Title:       title,
		CoverURL:    coverURL,
		VideoURL:    videoURL,
		OriginalURL: url,
		Author:      author,
		Platform:    "weibo",
	}

	return info, nil
}

func (w *WeiboAdapter) parseVideoPage(ctx context.Context, url string) (*VideoInfo, error) {
	fidRegex := regexp.MustCompile(`fid=(\d+:\d+)`)
	matches := fidRegex.FindStringSubmatch(url)
	if len(matches) < 2 {
		return nil, fmt.Errorf("无法提取视频ID")
	}
	fid := matches[1]

	apiURL := fmt.Sprintf("https://h5.video.weibo.com/api/component?page=/show&__rnd=%d", time.Now().UnixMilli())

	data := fmt.Sprintf(`data={"Component_Play_Playinfo":{"oid":"%s"}}`, fid)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://h5.video.weibo.com/show/"+fid)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	result := gjson.ParseBytes(bodyBytes)

	code := result.Get("code").String()
	if code != "100000" {
		msg := result.Get("msg").String()
		return nil, fmt.Errorf("微博API错误: %s", msg)
	}

	playInfo := result.Get("data.Component_Play_Playinfo")
	if !playInfo.Exists() {
		return nil, fmt.Errorf("未找到视频信息")
	}

	title := playInfo.Get("title").String()
	if title == "" {
		title = playInfo.Get("nickname").String() + " 的微博视频"
	}
	if title == "" {
		title = "微博视频"
	}

	author := playInfo.Get("nickname").String()

	coverURL := playInfo.Get("cover_image").String()
	if coverURL != "" {
		coverURL = strings.ReplaceAll(coverURL, "\\/", "/")
		if !strings.HasPrefix(coverURL, "http") {
			coverURL = "https:" + coverURL
		}
	}

	var videoURL string

	urls := playInfo.Get("urls")
	if urls.Exists() && urls.IsObject() {
		urls.ForEach(func(key, value gjson.Result) bool {
			label := key.String()
			videoURL = value.String()
			if strings.Contains(label, "高清") || strings.Contains(label, "720") {
				return false
			}
			return true
		})
	}

	if videoURL == "" {
		videoURL = playInfo.Get("stream_url").String()
	}

	if videoURL == "" {
		streamURLs := playInfo.Get("urls")
		if streamURLs.Exists() {
			videoURL = streamURLs.Array()[0].String()
		}
	}

	if videoURL == "" {
		return nil, fmt.Errorf("未找到视频播放地址")
	}

	videoURL = strings.ReplaceAll(videoURL, "\\/", "/")
	if !strings.HasPrefix(videoURL, "http") {
		videoURL = "https:" + videoURL
	}

	return &VideoInfo{
		Title:       title,
		CoverURL:    coverURL,
		VideoURL:    videoURL,
		OriginalURL: url,
		Author:      author,
		Platform:    "weibo",
	}, nil
}

func (w *WeiboAdapter) ExtractVideoID(url string) string {
	re := regexp.MustCompile(`(?:weibo\.com/tv/show|video\.weibo\.com/show)\?fid=(\d+:\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func (w *WeiboAdapter) FormatDuration(seconds int) string {
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (w *WeiboAdapter) BuildShareURL(videoID string) string {
	return fmt.Sprintf("https://video.weibo.com/show?fid=%s", videoID)
}

func (w *WeiboAdapter) BuildPlayerURL(videoID string) string {
	return w.BuildShareURL(videoID)
}

func (w *WeiboAdapter) GetPlatformName() string {
	return "微博"
}

func (w *WeiboAdapter) GetVideoInfo(ctx context.Context, url string) (*VideoInfo, error) {
	return w.Parse(ctx, url)
}

func (w *WeiboAdapter) Name() string {
	return "weibo"
}

type WeiboVideoResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}
