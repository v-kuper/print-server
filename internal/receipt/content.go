package receipt

type ContentSettings struct {
	Configured          bool `json:"configured"`
	ShowWeather         bool `json:"showWeather"`
	ShowWeatherAdvice   bool `json:"showWeatherAdvice"`
	ShowMotivationQuote bool `json:"showMotivationQuote"`
	ShowTonPortfolio    bool `json:"showTonPortfolio"`
	ShowUsdBynRate      bool `json:"showUsdBynRate"`
	ShowBankRates       bool `json:"showBankRates"`
	ShowMail            bool `json:"showMail"`
	ShowCalendar        bool `json:"showCalendar"`
	ShowHistory         bool `json:"showHistory"`
	ShowNews            bool `json:"showNews"`
	ShowDenisTrends     bool `json:"showDenisTrends"`
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
	}
}

func (s ContentSettings) Normalized() ContentSettings {
	if !s.Configured {
		return DefaultContentSettings()
	}
	return s
}
