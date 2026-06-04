package receipt

import (
	"atol-server/internal/denistrends"
	"atol-server/internal/news"
)

type ContentSettings struct {
	Configured          bool                  `json:"configured"`
	ShowWeather         bool                  `json:"showWeather"`
	ShowWeatherAdvice   bool                  `json:"showWeatherAdvice"`
	ShowMotivationQuote bool                  `json:"showMotivationQuote"`
	ShowDailyQuests     bool                  `json:"showDailyQuests"`
	ShowTonPortfolio    bool                  `json:"showTonPortfolio"`
	ShowOilPrice        bool                  `json:"showOilPrice"`
	ShowUsdBynRate      bool                  `json:"showUsdBynRate"`
	ShowBankRates       bool                  `json:"showBankRates"`
	ShowMail            bool                  `json:"showMail"`
	ShowCalendar        bool                  `json:"showCalendar"`
	ShowHistory         bool                  `json:"showHistory"`
	ShowNews            bool                  `json:"showNews"`
	ShowDenisTrends     bool                  `json:"showDenisTrends"`
	NewsSettings        *news.Settings        `json:"newsSettings,omitempty"`
	DenisTrendsSettings *denistrends.Settings `json:"denisTrendsSettings,omitempty"`
}

func DefaultContentSettings() ContentSettings {
	return ContentSettings{
		Configured:          true,
		ShowWeather:         true,
		ShowWeatherAdvice:   true,
		ShowMotivationQuote: true,
		ShowDailyQuests:     true,
		ShowTonPortfolio:    true,
		ShowOilPrice:        true,
		ShowUsdBynRate:      true,
		ShowBankRates:       true,
		ShowMail:            false,
		ShowCalendar:        true,
		ShowHistory:         true,
		ShowNews:            true,
		ShowDenisTrends:     false,
	}
}

func (s ContentSettings) Normalized() ContentSettings {
	if !s.Configured {
		return DefaultContentSettings()
	}
	if s.NewsSettings != nil {
		normalized := s.NewsSettings.Normalized()
		s.NewsSettings = &normalized
	}
	if s.DenisTrendsSettings != nil {
		normalized := s.DenisTrendsSettings.Normalized()
		s.DenisTrendsSettings = &normalized
	}
	return s
}
