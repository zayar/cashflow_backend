package utils

import (
	"os"
	"strings"
)

const (
	StorageProviderGCS      = "gcs"
	StorageProviderFirebase = "firebase"
	StorageProviderDO       = "do"
)

func GetStorageProvider() string {
	provider := strings.TrimSpace(strings.ToLower(os.Getenv("STORAGE_PROVIDER")))
	if provider == "" {
		return StorageProviderGCS
	}
	return provider
}
