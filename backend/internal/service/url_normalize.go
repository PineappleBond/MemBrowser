package service

import (
	"regexp"
	"strings"
)

var (
	// 匹配纯数字 ID
	numericIDRe = regexp.MustCompile(`/\d+(/|$)`)
	// 匹配 UUID
	uuidRe = regexp.MustCompile(`/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}(/|$)`)
)

func NormalizeURL(rawURL string) string {
	// 移除查询参数和 fragment
	if idx := strings.IndexAny(rawURL, "?#"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	// 替换纯数字 ID
	rawURL = numericIDRe.ReplaceAllString(rawURL, "/{id}$1")
	// 替换 UUID
	rawURL = uuidRe.ReplaceAllString(rawURL, "/{id}$1")
	return rawURL
}
