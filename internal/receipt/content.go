package receipt

import (
	"atol-server/internal/denistrends"
	"atol-server/internal/news"
)

const (
	DenisTrendsModeAuto  = "auto"
	DenisTrendsModeNow   = "now"
	DenisTrendsModeDay   = "day"
	DenisTrendsModeWeek  = "week"
	DenisTrendsModeMonth = "month"
)

type ContentSettings struct {
	Configured          bool                  `json:"configured"`
	ShowWeather         bool                  `json:"showWeather"`
	ShowWeatherAdvice   bool                  `json:"showWeatherAdvice"`
	ShowMotivationQuote bool                  `json:"showMotivationQuote"`
	ShowTonPortfolio    bool                  `json:"showTonPortfolio"`
	ShowUsdBynRate      bool                  `json:"showUsdBynRate"`
	ShowBankRates       bool                  `json:"showBankRates"`
	ShowMail            bool                  `json:"showMail"`
	ShowCalendar        bool                  `json:"showCalendar"`
	ShowHistory         bool                  `json:"showHistory"`
	ShowNews            bool                  `json:"showNews"`
	ShowDenisTrends     bool                  `json:"showDenisTrends"`
	DenisTrendsMode     string                `json:"denisTrendsMode,omitempty"`
	NewsSettings        *news.Settings        `json:"newsSettings,omitempty"`
	DenisTrendsSettings *denistrends.Settings `json:"denisTrendsSettings,omitempty"`
}

func DefaultContentSettings() ContentSettings {
	return ContentSettings{
		Configured:          true,
		ShowWeather:         true,
		ShowWeatherAdvice:   true,
		ShowMotivationQuote: true,
		ShowTonPortfolio:    true,
		ShowUsdBynRate:      true,
		ShowBankRates:       true,
		ShowMail:            false,
		ShowCalendar:        true,
		ShowHistory:         true,
		ShowNews:            true,
		ShowDenisTrends:     false,
		DenisTrendsMode:     DenisTrendsModeAuto,
	}
}

func (s ContentSettings) Normalized() ContentSettings {
	if !s.Configured {
		return DefaultContentSettings()
	}
	switch s.DenisTrendsMode {
	case DenisTrendsModeNow, DenisTrendsModeDay, DenisTrendsModeWeek, DenisTrendsModeMonth:
	default:
		s.DenisTrendsMode = DenisTrendsModeAuto
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
