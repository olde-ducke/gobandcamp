package main

import "strings"

// cache key = media url without any parameters
func getTruncatedURL(link string) string {
	if index := strings.Index(link, "?"); index > 0 {
		return link[:index]
	}
	return link
}
