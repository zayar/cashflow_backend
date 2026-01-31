package pitixsync

import "encoding/json"

type SyncModules struct {
	Customers      bool `json:"customers"`
	Items          bool `json:"items"`
	Invoices       bool `json:"invoices"`
	Taxes          bool `json:"taxes"`
	PaymentMethods bool `json:"paymentMethods"`
	Warehouses     bool `json:"warehouses"`
}

func DefaultModules() SyncModules {
	return SyncModules{
		Customers:      true,
		Items:          true,
		Invoices:       true,
		Taxes:          false,
		PaymentMethods: false,
		Warehouses:     false,
	}
}

func NormalizeModules(mod SyncModules) SyncModules {
	// Required modules must always be enabled.
	mod.Customers = true
	mod.Items = true
	mod.Invoices = true
	return mod
}

func DecodeModules(raw []byte) SyncModules {
	if len(raw) == 0 {
		return DefaultModules()
	}
	var mod SyncModules
	if err := json.Unmarshal(raw, &mod); err != nil {
		return DefaultModules()
	}
	return NormalizeModules(mod)
}

func EncodeModules(mod SyncModules) []byte {
	b, _ := json.Marshal(NormalizeModules(mod))
	return b
}

type CursorEntry struct {
	UpdatedSince string `json:"updated_since"`
	Cursor       string `json:"cursor"`
}

type CursorState struct {
	Customers CursorEntry `json:"customers"`
	Items     CursorEntry `json:"items"`
	Invoices  CursorEntry `json:"invoices"`
}

func DecodeCursorState(raw []byte) CursorState {
	if len(raw) == 0 {
		return CursorState{}
	}
	var state CursorState
	if err := json.Unmarshal(raw, &state); err != nil {
		return CursorState{}
	}
	return state
}

func EncodeCursorState(state CursorState) []byte {
	b, _ := json.Marshal(state)
	return b
}

type ConnectRequest struct {
	StoreId   string `json:"storeId"`
	StoreName string `json:"storeName"`
	APIKey    string `json:"apiKey"`
}

type UpdateSettingsRequest struct {
	Modules SyncModules `json:"modules"`
}

type TriggerSyncRequest struct {
	Modules SyncModules `json:"modules"`
}

type StatusResponse struct {
	Connection       ConnectionResponse `json:"connection"`
	LastSyncAt       *string            `json:"lastSyncAt"`
	LastSuccessSyncAt *string           `json:"lastSuccessSyncAt"`
	Modules          SyncModules         `json:"modules"`
}

type ConnectionResponse struct {
	Status     string `json:"status"`
	MerchantId string `json:"merchantId"`
	StoreName  string `json:"storeName"`
}

type SyncHistoryResponse struct {
	Items []SyncRunResponse `json:"items"`
}

type SyncRunResponse struct {
	ID            uint   `json:"id"`
	Status        string `json:"status"`
	StartedAt     *string `json:"startedAt"`
	FinishedAt    *string `json:"finishedAt"`
	DurationMs    int64  `json:"durationMs"`
	RecordsSynced int    `json:"recordsSynced"`
	ErrorCount    int    `json:"errorCount"`
	TriggeredBy   string `json:"triggeredBy"`
}

type SyncRunDetailResponse struct {
	SyncRunResponse
	Errors []SyncErrorResponse `json:"errors"`
}

type SyncErrorResponse struct {
	ID         uint   `json:"id"`
	EntityType string `json:"entityType"`
	ExternalId string `json:"externalId"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
}

type PubSubPushEnvelope struct {
	Message struct {
		Data []byte `json:"data"`
		ID   string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

type SyncPubSubPayload struct {
	RunId       uint   `json:"run_id"`
	BusinessId  string `json:"business_id"`
	ConnectionId uint  `json:"connection_id"`
}
