package adapter

import (
	"context"
)

type VideoInfo struct {
	Title       string `json:"title"`
	CoverURL    string `json:"cover_url"`
	VideoURL    string `json:"video_url"`
	OriginalURL string `json:"original_url"`
	Duration    int    `json:"duration"`
	FileSize    int64  `json:"file_size"`
	Author      string `json:"author"`
	Platform    string `json:"platform"`
}

type Adapter interface {
	Parse(ctx context.Context, url string) (*VideoInfo, error)
	SupportedDomains() []string
	Name() string
}

func GetAdapter(platform string) Adapter {
	switch platform {
	case "douyin":
		return &DouyinAdapter{}
	case "kuaishou":
		return &KuaishouAdapter{}
	case "xiaohongshu":
		return &XiaohongshuAdapter{}
	case "bilibili":
		return &BilibiliAdapter{}
	case "weibo":
		return &WeiboAdapter{}
	case "xigua":
		return &XiguaAdapter{}
	case "youtube":
		return &YouTubeAdapter{}
	case "tiktok":
		return &TikTokAdapter{}
	default:
		return nil
	}
}

func GetAllAdapters() []struct {
	Name     string
	Platform string
	Domains  []string
}{
	return []struct {
		Name     string
		Platform string
		Domains  []string
	}{
		{"抖音", "douyin", (&DouyinAdapter{}).SupportedDomains()},
		{"快手", "kuaishou", (&KuaishouAdapter{}).SupportedDomains()},
		{"小红书", "xiaohongshu", (&XiaohongshuAdapter{}).SupportedDomains()},
		{"B站", "bilibili", (&BilibiliAdapter{}).SupportedDomains()},
		{"微博", "weibo", (&WeiboAdapter{}).SupportedDomains()},
		{"西瓜视频", "xigua", (&XiguaAdapter{}).SupportedDomains()},
		{"YouTube", "youtube", (&YouTubeAdapter{}).SupportedDomains()},
		{"TikTok", "tiktok", (&TikTokAdapter{}).SupportedDomains()},
	}
}
