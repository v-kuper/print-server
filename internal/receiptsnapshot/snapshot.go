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

type Snapshot struct {
	ID          string
	WorkspaceID string
	Status      Status
	NewsItems   []NewsItem
	Error       string
	CreatedAt   time.Time
	PublishedAt *time.Time
	FailedAt    *time.Time
}
