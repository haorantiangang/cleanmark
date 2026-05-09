package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type YouTubeAdapter struct{}

func (y *YouTubeAdapter) Name() string {
	return "YouTube"
}

func (y *YouTubeAdapter) SupportedDomains() []string {
	return []string{"youtube.com", "youtu.be", "m.youtube.com"}
}

func (y *YouTubeAdapter) Parse(ctx context.Context, inputURL string) (*VideoInfo, error) {
	videoID := y.extractVideoID(inputURL)
	if videoID == "" {
		return nil, fmt.Errorf("无效的YouTube链接格式")
	}

	apiURL := fmt.Sprintf(
		"https://www.youtube.com/get_video_info?video_id=%s&el=detailpage",
		videoID,
	)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建API请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求YouTube API失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	queryParams, _ := url.ParseQuery(string(bodyBytes))

	playerResponse := queryParams.Get("player_response")
	if playerResponse == "" {
		return nil, fmt.Errorf("获取视频信息失败，视频可能不可用或受地区限制")
	}

	var pr struct {
		PlayabilityStatus struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"playabilityStatus"`
		VideoDetails struct {
			Title     string `json:"title"`
			Author     string `json:"author"`
			LengthSec  int    `json:"lengthSeconds"`
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnail"`
		} `json:"videoDetails"`
		StreamingData struct {
			Formats []VideoFormat `json:"formats"`
			AdaptiveFormats []VideoFormat `json:"adaptiveFormats"`
		} `json:"streamingData"`
	}

	if err := json.Unmarshal([]byte(playerResponse), &pr); err != nil {
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	if pr.PlayabilityStatus.Status != "ok" {
		reason := pr.PlayabilityStatus.Reason
		if reason == "" {
			reason = "视频不可用或受限"
		}
		return nil, fmt.Errorf(reason)
	}

	bestFormat := y.selectBestQuality(pr.StreamingData.Formats, pr.StreamingData.AdaptiveFormats)
	if bestFormat == nil {
		return nil, fmt.Errorf("未找到可用的视频流")
	}

	var coverURL string
	if len(pr.VideoDetails.Thumbnails) > 0 {
		coverURL = pr.VideoDetails.Thumbnails[len(pr.VideoDetails.Thumbnails)-1].URL
	}

	info := &VideoInfo{
		Title:       pr.VideoDetails.Title,
		CoverURL:    coverURL,
		VideoURL:    bestFormat.URL,
		OriginalURL: inputURL,
		Duration:    pr.VideoDetails.LengthSec,
		Author:      pr.VideoDetails.Author,
		Platform:    "youtube",
	}

	return info, nil
}

func (y *YouTubeAdapter) extractVideoID(inputURL string) string {
	inputURL = strings.TrimSpace(inputURL)
	
	patterns := []string{
		`(?:youtube\.com/watch\?.*v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/|youtube\.com/shorts/)([a-zA-Z0-9_-]{11})`,
		`^([a-zA-Z0-9_-]{11})$`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(inputURL)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

type VideoFormat struct {
	Itag      int    `json:"itag"`
	MimeType  string `json:"mimeType"`
	Quality   string `json:"qualityLabel"`
	URL       string `json:"url"`
	Bitrate   int    `json:"bitrate"`
}

func (y *YouTubeAdapter) selectBestQuality(formats, adaptiveFormats []VideoFormat) *VideoFormat {

	allFormats := make([]VideoFormat, 0, len(formats)+len(adaptiveFormats))
	allFormats = append(allFormats, formats...)
	for _, f := range adaptiveFormats {
		allFormats = append(allFormats, VideoFormat{
			Itag:     f.Itag,
			MimeType: f.MimeType,
			Quality:  f.Quality,
			URL:      f.URL,
			Bitrate:  f.Bitrate,
		})
	}

	var bestFormat *VideoFormat

	maxBitrate := 0

	for i := range allFormats {
		fmt := &allFormats[i]
		
		if !strings.HasPrefix(fmt.MimeType, "video/") {
			continue
		}
		
		if fmt.URL == "" {
			continue
		}
		
		if bestFormat == nil || fmt.Bitrate > maxBitrate {
			bestFormat = fmt
			maxBitrate = fmt.Bitrate
		}
	}

	return bestFormat
}
