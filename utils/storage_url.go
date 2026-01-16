package utils

import (
	"net/url"
	"os"
	"strings"
)

func BuildObjectAccessURL(objectKey string) string {
	base := strings.TrimSpace(os.Getenv("STORAGE_ACCESS_BASE_URL"))
	if base != "" {
		if strings.Contains(base, "{objectKey}") {
			escaped := objectKey
			if strings.Contains(base, "?") {
				escaped = url.QueryEscape(objectKey)
			}
			return strings.ReplaceAll(base, "{objectKey}", escaped)
		}
		if strings.Contains(base, "?") {
			return base + url.QueryEscape(objectKey)
		}
		return strings.TrimRight(base, "/") + "/" + objectKey
	}

	gcsURL := strings.TrimSpace(os.Getenv("GCS_URL"))
	gcsBucket := strings.TrimSpace(os.Getenv("GCS_BUCKET"))
	if gcsURL != "" && gcsBucket != "" {
		return "https://" + gcsURL + "/" + gcsBucket + "/" + objectKey
	}

	return objectKey
}

func ExtractObjectKeyFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	if strings.HasPrefix(rawURL, "gs://") {
		rawURL = strings.TrimPrefix(rawURL, "gs://")
		parts := strings.SplitN(rawURL, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err == nil {
		if key := parsed.Query().Get("key"); key != "" {
			return key
		}
		if key := parsed.Query().Get("objectKey"); key != "" {
			return key
		}
	}

	gcsURL := strings.TrimSpace(os.Getenv("GCS_URL"))
	gcsBucket := strings.TrimSpace(os.Getenv("GCS_BUCKET"))
	if gcsURL != "" && gcsBucket != "" {
		for _, scheme := range []string{"https://", "http://"} {
			prefix := scheme + gcsURL + "/" + gcsBucket + "/"
			if strings.HasPrefix(rawURL, prefix) {
				return strings.TrimPrefix(rawURL, prefix)
			}
		}
	}

	spURL := strings.TrimSpace(os.Getenv("SP_URL"))
	spBucket := strings.TrimSpace(os.Getenv("SP_BUCKET"))
	if spURL != "" && spBucket != "" {
		prefix := "https://" + spBucket + "." + spURL + "/"
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}

	// Handle access base URL with placeholder or trailing query key.
	base := strings.TrimSpace(os.Getenv("STORAGE_ACCESS_BASE_URL"))
	if base != "" {
		if strings.Contains(base, "{objectKey}") {
			parts := strings.Split(base, "{objectKey}")
			if len(parts) == 2 && strings.HasPrefix(rawURL, parts[0]) && strings.HasSuffix(rawURL, parts[1]) {
				trimmed := strings.TrimSuffix(strings.TrimPrefix(rawURL, parts[0]), parts[1])
				if decoded, err := url.QueryUnescape(trimmed); err == nil {
					return decoded
				}
				return trimmed
			}
		}
		if strings.Contains(base, "?") && strings.Contains(rawURL, base) {
			trimmed := strings.TrimPrefix(rawURL, base)
			if decoded, err := url.QueryUnescape(trimmed); err == nil {
				return decoded
			}
			return trimmed
		}
	}

	return ""
}
