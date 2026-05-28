package receiptsnapshot

import (
	"net/url"
	"strings"
)

type Settings struct {
	BaseURL string `json:"baseUrl"`
}

func DefaultSettings() Settings {
	return Settings{}
}

func (s Settings) Normalized() Settings {
	s.BaseURL = strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	return s
}

func (s Settings) Validate() error {
	normalized := s.Normalized()
	if normalized.BaseURL == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(normalized.BaseURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &url.Error{Op: "parse", URL: normalized.BaseURL, Err: errUnsupportedScheme}
	}
	if parsed.Host == "" {
		return &url.Error{Op: "parse", URL: normalized.BaseURL, Err: errMissingHost}
	}
	return nil
}

func (s Settings) SnapshotURL(id string) (string, bool) {
	normalized := s.Normalized()
	id = strings.TrimSpace(id)
	if normalized.BaseURL == "" || id == "" || normalized.Validate() != nil {
		return "", false
	}
	value, err := url.JoinPath(normalized.BaseURL, "snapshots", id)
	if err != nil {
		return "", false
	}
	return value, true
}
