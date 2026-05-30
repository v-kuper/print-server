package motivation

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"atol-server/internal/dailyquest"
)

const (
	DefaultBaseURL  = "http://host.docker.internal:11434"
	DefaultModel    = "gemma4:31b-cloud"
	DefaultTimezone = "Europe/Minsk"
)

type Settings struct {
	Configured        bool                    `json:"configured,omitempty"`
	Enabled           bool                    `json:"enabled"`
	BaseURL           string                  `json:"baseUrl"`
	Model             string                  `json:"model"`
	CacheDate         string                  `json:"cacheDate,omitempty"`
	CachedQuote       string                  `json:"cachedQuote,omitempty"`
	QuestCacheDate    string                  `json:"questCacheDate,omitempty"`
	CachedDailyQuests []dailyquest.DailyQuest `json:"cachedDailyQuests,omitempty"`
	LastError         string                  `json:"lastError,omitempty"`
}

type Quote struct {
	Text string `json:"text"`
}

func DefaultSettings() Settings {
	return Settings{
		Configured: true,
		Enabled:    true,
		BaseURL:    DefaultBaseURL,
		Model:      DefaultModel,
	}
}

func (s Settings) Normalized() Settings {
	if !s.Configured &&
		strings.TrimSpace(s.BaseURL) == "" &&
		strings.TrimSpace(s.Model) == "" &&
		strings.TrimSpace(s.CacheDate) == "" &&
		strings.TrimSpace(s.CachedQuote) == "" &&
		strings.TrimSpace(s.QuestCacheDate) == "" &&
		len(s.CachedDailyQuests) == 0 &&
		strings.TrimSpace(s.LastError) == "" {
		return DefaultSettings()
	}

	normalized := s
	normalized.Configured = true
	normalized.Enabled = true
	normalized.BaseURL = strings.TrimRight(strings.TrimSpace(normalized.BaseURL), "/")
	if normalized.BaseURL == "" {
		normalized.BaseURL = DefaultBaseURL
	}
	normalized.Model = strings.TrimSpace(normalized.Model)
	if normalized.Model == "" {
		normalized.Model = DefaultModel
	}
	normalized.CacheDate = strings.TrimSpace(normalized.CacheDate)
	normalized.CachedQuote = sanitizeQuote(normalized.CachedQuote)
	normalized.QuestCacheDate = strings.TrimSpace(normalized.QuestCacheDate)
	normalized.CachedDailyQuests = sanitizeDailyQuests(normalized.CachedDailyQuests)
	normalized.LastError = strings.TrimSpace(normalized.LastError)
	return normalized
}

func (s Settings) Validate() error {
	normalized := s.Normalized()
	if _, err := url.ParseRequestURI(normalized.BaseURL); err != nil {
		return fmt.Errorf("motivation base URL is invalid")
	}
	if strings.TrimSpace(normalized.Model) == "" {
		return fmt.Errorf("motivation model is required")
	}
	return nil
}

func CacheDate(now time.Time) string {
	location, err := time.LoadLocation(DefaultTimezone)
	if err == nil {
		now = now.In(location)
	}
	return now.Format("2006-01-02")
}

func sanitizeQuote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, `"'«»“”`)
	value = strings.ReplaceAll(value, "**", "")
	value = strings.ReplaceAll(value, "*", "")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'«»“”`)
	return strings.TrimSpace(value)
}

func sanitizeDailyQuests(quests []dailyquest.DailyQuest) []dailyquest.DailyQuest {
	if len(quests) == 0 {
		return nil
	}
	result := make([]dailyquest.DailyQuest, 0, 3)
	for _, quest := range quests {
		text := sanitizeQuote(quest.Text)
		if quest.ID == 0 || text == "" {
			continue
		}
		result = append(result, dailyquest.DailyQuest{ID: quest.ID, Text: text})
		if len(result) == 3 {
			break
		}
	}
	return result
}
