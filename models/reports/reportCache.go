package reports

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

func reportCacheEnabled() bool {
	v := strings.TrimSpace(os.Getenv("ENABLE_REPORT_CACHE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on")
}

func reportCacheTTL() time.Duration {
	// Env: REPORT_CACHE_TTL_SECONDS (default 120s)
	ttl := 120
	if v := strings.TrimSpace(os.Getenv("REPORT_CACHE_TTL_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = n
		}
	}
	return time.Duration(ttl) * time.Second
}

func reportSlowMs() int64 {
	// Env: REPORT_SLOW_MS (default 500ms)
	ms := int64(500)
	if v := strings.TrimSpace(os.Getenv("REPORT_SLOW_MS")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			ms = n
		}
	}
	return ms
}

func logSlowReport(ctx context.Context, name string, started time.Time, extra map[string]any) {
	d := time.Since(started)
	if d.Milliseconds() < reportSlowMs() {
		return
	}
	biz, _ := utils.GetBusinessIdFromContext(ctx)
	cid, _ := utils.GetCorrelationIdFromContext(ctx)
	log.Printf("slow_report name=%s ms=%d business_id=%s correlation_id=%s extra=%v", name, d.Milliseconds(), biz, cid, extra)
}

func cacheGet[T any](key string, dest *T) (bool, error) {
	return config.GetRedisObject(key, dest)
}

func cacheSet(key string, obj any, ttl time.Duration) error {
	return config.SetRedisObject(key, obj, ttl)
}

