package models

import "time"

const (
	IntegrationProviderPitiX = "pitix"
)

const (
	IntegrationStatusConnected    = "connected"
	IntegrationStatusDisconnected = "disconnected"
	IntegrationStatusError        = "error"
)

const (
	SyncRunStatusQueued  = "queued"
	SyncRunStatusRunning = "running"
	SyncRunStatusSuccess = "success"
	SyncRunStatusFailed  = "failed"
	SyncRunStatusPartial = "partial"
)

const (
	SyncTriggeredManual = "manual"
	SyncTriggeredRetry  = "retry"
	SyncTriggeredSystem = "system"
)

type IntegrationConnection struct {
	ID               uint       `gorm:"primary_key" json:"id"`
	BusinessId       string     `gorm:"index;not null" json:"business_id"`
	Provider         string     `gorm:"index;size:50;not null" json:"provider"`
	Status           string     `gorm:"size:20;not null" json:"status"`
	AuthType         string     `gorm:"size:20" json:"auth_type"`
	AuthSecretRef    string     `gorm:"type:text" json:"auth_secret_ref"`
	StoreId          string     `gorm:"size:100" json:"store_id"`
	StoreName        string     `gorm:"size:255" json:"store_name"`
	SettingsJSON     []byte     `gorm:"type:json" json:"settings"`
	CursorStateJSON  []byte     `gorm:"type:json" json:"cursor_state"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
	LastSuccessSyncAt *time.Time `json:"last_success_sync_at"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type IntegrationSyncRun struct {
	ID             uint       `gorm:"primary_key" json:"id"`
	BusinessId     string     `gorm:"index;not null" json:"business_id"`
	ConnectionId   uint       `gorm:"index;not null" json:"connection_id"`
	Provider       string     `gorm:"index;size:50;not null" json:"provider"`
	Status         string     `gorm:"size:20;not null" json:"status"`
	TriggeredBy    string     `gorm:"size:20" json:"triggered_by"`
	ModulesJSON    []byte     `gorm:"type:json" json:"modules"`
	StatsJSON      []byte     `gorm:"type:json" json:"stats"`
	CursorStateJSON []byte    `gorm:"type:json" json:"cursor_state"`
	RecordsSynced  int        `json:"records_synced"`
	ErrorCount     int        `json:"error_count"`
	ParentRunId    *uint      `gorm:"index" json:"parent_run_id"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	DurationMs     int64      `json:"duration_ms"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type IntegrationEntityMapping struct {
	ID           uint       `gorm:"primary_key" json:"id"`
	BusinessId   string     `gorm:"uniqueIndex:idx_integration_mapping,priority:1;not null" json:"business_id"`
	ConnectionId uint       `gorm:"index;not null" json:"connection_id"`
	Provider     string     `gorm:"uniqueIndex:idx_integration_mapping,priority:2;size:50;not null" json:"provider"`
	EntityType   string     `gorm:"uniqueIndex:idx_integration_mapping,priority:3;size:50;not null" json:"entity_type"`
	ExternalId   string     `gorm:"uniqueIndex:idx_integration_mapping,priority:4;size:128;not null" json:"external_id"`
	InternalId   string     `gorm:"size:128;not null" json:"internal_id"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	MetadataJSON []byte     `gorm:"type:json" json:"metadata"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type IntegrationSyncError struct {
	ID         uint      `gorm:"primary_key" json:"id"`
	SyncRunId  uint      `gorm:"index;not null" json:"sync_run_id"`
	BusinessId string    `gorm:"index;not null" json:"business_id"`
	EntityType string    `gorm:"size:50" json:"entity_type"`
	ExternalId string    `gorm:"size:128" json:"external_id"`
	ErrorCode  string    `gorm:"size:64" json:"error_code"`
	Message    string    `gorm:"type:text" json:"message"`
	PayloadJSON []byte   `gorm:"type:json" json:"payload"`
	Retryable  bool      `gorm:"default:false" json:"retryable"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
