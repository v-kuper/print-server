package receiptsnapshot

import (
	"errors"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"
)

var (
	ErrNotFound          = errors.New("receipt snapshot not found")
	errUnsupportedScheme = errors.New("base URL scheme must be http or https")
	errMissingHost       = errors.New("base URL host is required")
)

type NewsItem struct {
	Title         string `json:"title"`
	OriginalTitle string `json:"originalTitle,omitempty"`
	SourceName    string `json:"sourceName"`
	Link          string `json:"link,omitempty"`
}

type ReceiptLine struct {
	Text         string `json:"text"`
	Alignment    string `json:"alignment,omitempty"`
	Role         string `json:"role,omitempty"`
	Font         int    `json:"font,omitempty"`
	DoubleWidth  bool   `json:"doubleWidth,omitempty"`
	DoubleHeight bool   `json:"doubleHeight,omitempty"`
}

type CreateInput struct {
	NewsItems    []NewsItem
	ReceiptLines []ReceiptLine
}

type Snapshot struct {
	ID           string
	WorkspaceID  string
	Status       Status
	NewsItems    []NewsItem
	ReceiptLines []ReceiptLine
	Error        string
	CreatedAt    time.Time
	PublishedAt  *time.Time
	FailedAt     *time.Time
}
