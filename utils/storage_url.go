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

	// Allow passing raw object keys directly (e.g. "businessId/products/logo.png").
	// This keeps delete flows working even when BuildObjectAccessURL returns the key
	// (missing STORAGE_ACCESS_BASE_URL / GCS_URL envs).
	if !strings.Contains(rawURL, "://") && !strings.HasPrefix(rawURL, "/") && strings.Contains(rawURL, "/") {
		// Basic hardening: reject path traversal.
		if strings.Contains(rawURL, "..") {
			return ""
		}
		return rawURL
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

		// Handle common Google Cloud Storage URL formats even when env vars are missing.
		// Examples:
		// - https://storage.googleapis.com/<bucket>/<objectKey>
		// - https://<bucket>.storage.googleapis.com/<objectKey>
		// - https://storage.cloud.google.com/<bucket>/<objectKey>
		host := strings.ToLower(strings.TrimSpace(parsed.Host))
		p := strings.TrimPrefix(parsed.Path, "/")
		if host == "storage.googleapis.com" || host == "storage.cloud.google.com" {
			parts := strings.SplitN(p, "/", 2)
			if len(parts) == 2 && parts[1] != "" {
				return parts[1]
			}
		}
		if strings.HasSuffix(host, ".storage.googleapis.com") {
			// bucket is in host; object key is the full path
			if p != "" {
				return p
			}
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
