package codeup

import (
	"fmt"
	"strings"
	"time"
)

func isTruthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

// timeFormats 列出 Codeup API 可能返回的时间格式
var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05+08:00",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.000Z",
}

// RelativeTime 将时间字符串转换为相对时间描述（如 "3 个月前"）
func RelativeTime(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	var t time.Time
	var err error
	for _, layout := range timeFormats {
		t, err = time.Parse(layout, dateStr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return dateStr
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "刚刚"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		return fmt.Sprintf("%d 分钟前", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d 小时前", hours)
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d 天前", days)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months < 1 {
			months = 1
		}
		return fmt.Sprintf("%d 个月前", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years < 1 {
			years = 1
		}
		return fmt.Sprintf("%d 年前", years)
	}
}
