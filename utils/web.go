package utils

import (
	"net/url"
	"path"
	"strings"
)

func GetFileNameFromURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return path.Base(u.Path)
}

func GetQSValue(urlStr string, key string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	values := u.Query()
	return values.Get(key)
}

func SplitURL(urlStr string) (baseURL, fileURL string) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", ""
	}
	fileURL = u.Scheme + "://" + u.Host + u.Path
	baseURL = fileURL[:strings.LastIndex(fileURL, "/")]
	return baseURL, fileURL
}

func ExtractValue(s, key string) string {
	parts := strings.Split(s, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, key) {
			return strings.TrimPrefix(part, key)
		}
	}
	return ""
}

func JoinURL(baseURL, relativePath string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	relative, err := url.Parse(relativePath)
	if err != nil {
		return ""
	}

	return base.ResolveReference(relative).String()
}
