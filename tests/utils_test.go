package tests

import (
	"cleanmark/internal/utils"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"抖音视频", "https://www.douyin.com/video/123456", "douyin"},
		{"抖音分享", "https://v.douyin.com/abc123", "douyin"},
		{"快手视频", "https://www.kuaishou.com/video/xxx", "kuaishou"},
		{"快手短链", "https://v.gifshow.com/xxx", "kuaishou"},
		{"小红书笔记", "https://www.xiaohongshu.com/explore/xxx", "xiaohongshu"},
		{"小红书短链", "https://xhslink.com/xxx", "xiaohongshu"},
		{"B站视频", "https://www.bilibili.com/video/BV1xx411c7mD", "bilibili"},
		{"B站短链", "https://b23.tv/abc123", "bilibili"},
		{"微博内容", "https://weibo.com/xxx/status/123456", "weibo"},
		{"西瓜视频", "https://www.ixigua.com/xxx", "xigua"},
		{"YouTube视频", "https://www.youtube.com/watch?v=abc123", "youtube"},
		{"YouTube短链", "https://youtu.be/abc123", "youtube"},
		{"TikTok视频", "https://www.tiktok.com/@user/video/123", "tiktok"},
		{"TikTok短链", "https://vm.tiktok.com/xxx", "tiktok"},
		{"头条链接", "https://www.toutiao.com/xxx", "xigua"},
		{"未知平台", "https://example.com/video/123", ""},
		{"空字符串", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.DetectPlatform(tt.url)
			if result != tt.expected {
				t.Errorf("DetectPlatform(%s) = %s, want %s", tt.url, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{0, "00:00"},
		{30, "00:30"},
		{60, "01:00"},
		{90, "01:30"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
		{86400, "24:00:00"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := utils.FormatDuration(tt.seconds)
			if result != tt.expected {
				t.Errorf("FormatDuration(%d) = %s, want %s", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := utils.FormatFileSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatFileSize(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGenerateOrderNo(t *testing.T) {
	orderNo1 := utils.GenerateOrderNo()
	orderNo2 := utils.GenerateOrderNo()

	if len(orderNo1) == 0 || len(orderNo2) == 0 {
		t.Error("订单号不能为空")
	}

	if orderNo1 == orderNo2 {
		t.Error("两次生成的订单号应该不同")
	}

	if orderNo1[:2] != "CM" {
		t.Errorf("订单号前缀应该是CM，得到: %s", orderNo1[:2])
	}
}

func TestRandomString(t *testing.T) {
	str1 := utils.RandomString(10)
	str2 := utils.RandomString(10)

	if len(str1) != 10 || len(str2) != 10 {
		t.Error("随机字符串长度不正确")
	}

	if str1 == str2 {
		t.Error("两次生成的随机字符串应该不同")
	}
}

func TestGetPlatformName(t *testing.T) {
	tests := []struct {
		platform string
		expected string
	}{
		{"douyin", "抖音"},
		{"kuaishou", "快手"},
		{"xiaohongshu", "小红书"},
		{"bilibili", "B站"},
		{"weibo", "微博"},
		{"xigua", "西瓜视频"},
		{"youtube", "YouTube"},
		{"tiktok", "TikTok"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			result := utils.GetPlatformName(tt.platform)
			if result != tt.expected {
				t.Errorf("GetPlatformName(%s) = %s, want %s", tt.platform, result, tt.expected)
			}
		})
	}
}
