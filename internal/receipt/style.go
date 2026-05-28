package receipt

const maxPrinterFont = 9

type StyleSettings struct {
	Configured              bool      `json:"configured"`
	NormalFont              int       `json:"normalFont"`
	EmphasisFont            int       `json:"emphasisFont"`
	CalendarFont            int       `json:"calendarFont"`
	TemperatureFont         int       `json:"temperatureFont"`
	CalendarDoubleWidth     bool      `json:"calendarDoubleWidth"`
	CalendarDoubleHeight    bool      `json:"calendarDoubleHeight"`
	TemperatureDoubleWidth  bool      `json:"temperatureDoubleWidth"`
	TemperatureDoubleHeight bool      `json:"temperatureDoubleHeight"`
	CalendarAlignment       Alignment `json:"calendarAlignment"`
	TemperatureAlignment    Alignment `json:"temperatureAlignment"`
	NormalAlignment         Alignment `json:"normalAlignment"`
}

type lineStyle struct {
	Role         Role
	Font         int
	DoubleWidth  bool
	DoubleHeight bool
	Alignment    Alignment
}

func DefaultStyleSettings() StyleSettings {
	return StyleSettings{
		Configured:              true,
		NormalFont:              0,
		EmphasisFont:            0,
		CalendarFont:            0,
		TemperatureFont:         0,
		CalendarDoubleWidth:     true,
		CalendarDoubleHeight:    true,
		TemperatureDoubleWidth:  true,
		TemperatureDoubleHeight: true,
		CalendarAlignment:       AlignmentCenter,
		TemperatureAlignment:    AlignmentCenter,
		NormalAlignment:         AlignmentCenter,
	}
}

func (s StyleSettings) Normalized() StyleSettings {
	if !s.Configured {
		return DefaultStyleSettings()
	}
	s.NormalFont = clampFont(s.NormalFont)
	s.EmphasisFont = clampFont(s.EmphasisFont)
	if s.CalendarFont == 0 && s.EmphasisFont != 0 {
		s.CalendarFont = s.EmphasisFont
	}
	if s.TemperatureFont == 0 && s.EmphasisFont != 0 {
		s.TemperatureFont = s.EmphasisFont
	}
	s.CalendarFont = clampFont(s.CalendarFont)
	s.TemperatureFont = clampFont(s.TemperatureFont)
	if s.CalendarAlignment == "" {
		s.CalendarAlignment = AlignmentCenter
	}
	if s.TemperatureAlignment == "" {
		s.TemperatureAlignment = AlignmentCenter
	}
	if s.NormalAlignment == "" {
		s.NormalAlignment = AlignmentCenter
	}
	return s
}

func (s StyleSettings) normalLineStyle() lineStyle {
	normalized := s.Normalized()
	return lineStyle{
		Role:      RoleNormal,
		Font:      normalized.NormalFont,
		Alignment: normalized.NormalAlignment,
	}
}

func (s StyleSettings) calendarLineStyle() lineStyle {
	normalized := s.Normalized()
	return lineStyle{
		Role:         RoleCalendar,
		Font:         normalized.CalendarFont,
		DoubleWidth:  normalized.CalendarDoubleWidth,
		DoubleHeight: normalized.CalendarDoubleHeight,
		Alignment:    normalized.CalendarAlignment,
	}
}

func (s StyleSettings) temperatureLineStyle() lineStyle {
	normalized := s.Normalized()
	return lineStyle{
		Role:         RoleTemperature,
		Font:         normalized.TemperatureFont,
		DoubleWidth:  normalized.TemperatureDoubleWidth,
		DoubleHeight: normalized.TemperatureDoubleHeight,
		Alignment:    normalized.TemperatureAlignment,
	}
}

func (s StyleSettings) originalLineStyle() lineStyle {
	return lineStyle{
		Role: RoleOriginal,
		Font: 1,
	}
}

func clampFont(font int) int {
	switch {
	case font < 0:
		return 0
	case font > maxPrinterFont:
		return maxPrinterFont
	default:
		return font
	}
}
