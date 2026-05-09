package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

func GenerateOrderNo() string {
	timestamp := time.Now().Format("20060102150405")
	random := randomString(6)
	return fmt.Sprintf("CM%s%s", timestamp, random)
}

func RandomString(length int) string {
	return randomString(length)
}

func randomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func DetectPlatform(url string) string {
	url = strings.ToLower(url)
	
	switch {
	case strings.Contains(url, "iesdouyin.com/xg/video"):
		return "xigua"
	case strings.Contains(url, "douyin.com") || strings.Contains(url, "iesdouyin.com"):
		return "douyin"
	case strings.Contains(url, "kuaishou.com") || strings.Contains(url, "gifshow.com"):
		return "kuaishou"
	case strings.Contains(url, "xiaohongshu.com") || strings.Contains(url, "xhslink.com"):
		return "xiaohongshu"
	case strings.Contains(url, "bilibili.com") || strings.Contains(url, "b23.tv"):
		return "bilibili"
	case strings.Contains(url, "weibo.com") || strings.Contains(url, "weibo.cn"):
		return "weibo"
	case strings.Contains(url, "ixigua.com") || strings.Contains(url, "toutiao.com"):
		return "xigua"
	case strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be"):
		return "youtube"
	case strings.Contains(url, "tiktok.com") || strings.Contains(url, "vm.tiktok.com"):
		return "tiktok"
	default:
		return ""
	}
}

func FormatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func GetPlatformName(platform string) string {
	names := map[string]string{
		"douyin":      "抖音",
		"kuaishou":    "快手",
		"xiaohongshu": "小红书",
		"bilibili":    "B站",
		"weibo":       "微博",
		"xigua":       "西瓜视频",
		"youtube":     "YouTube",
		"tiktok":      "TikTok",
	}
	
	if name, ok := names[platform]; ok {
		return name
	}
	return platform
}

func ExtractURL(text string) string {
	urlPattern := `https?://[^\s<>"{}|\\^\[\]]+`
	re := regexp.MustCompile(urlPattern)
	matches := re.FindAllString(text, -1)
	
	if len(matches) > 0 {
		return strings.TrimSpace(matches[0])
	}
	
	return strings.TrimSpace(text)
}
