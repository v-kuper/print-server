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
	Text              string  `json:"text,omitempty"`
	Link              string  `json:"link,omitempty"`
	QRCode            string  `json:"qrCode,omitempty"`
	ImageKey          string  `json:"imageKey,omitempty"`
	ImageURL          string  `json:"imageUrl,omitempty"`
	ImageWidth        int     `json:"imageWidth,omitempty"`
	ImageHeight       int     `json:"imageHeight,omitempty"`
	ImageScalePercent int     `json:"imageScalePercent,omitempty"`
	Alignment         string  `json:"alignment,omitempty"`
	Role              string  `json:"role,omitempty"`
	Font              int     `json:"font,omitempty"`
	DoubleWidth       bool    `json:"doubleWidth,omitempty"`
	DoubleHeight      bool    `json:"doubleHeight,omitempty"`
	LineSize          float64 `json:"lineSize,omitempty"`
}

type CreateInput struct {
	NewsItems    []NewsItem
	ReceiptLines []ReceiptLine
	PaperChars   int
}

type Snapshot struct {
	ID           string
	WorkspaceID  string
	Status       Status
	NewsItems    []NewsItem
	ReceiptLines []ReceiptLine
	PaperChars   int
	Error        string
	CreatedAt    time.Time
	PublishedAt  *time.Time
	FailedAt     *time.Time
}
