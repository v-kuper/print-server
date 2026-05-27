package motivation

import (
	"fmt"
	"math"
	"strings"
	"time"

	weatherdata "atol-server/internal/weather"
)

type WeatherAdvice struct {
	Text string `json:"text"`
}

type WeatherContext struct {
	LocationName                   string
	ObservedAt                     time.Time
	Condition                      string
	TemperatureC                   float64
	ApparentTemperatureC           *float64
	RelativeHumidityPct            *float64
	PrecipitationMm                *float64
	WindSpeedMs                    *float64
	WindGustsMs                    *float64
	WindDirectionDeg               *float64
	UVIndex                        *float64
	UVIndexMax                     *float64
	PrecipitationProbabilityMaxPct *float64
	VisibilityM                    *float64
	DewPointC                      *float64
	SurfacePressureHpa             *float64
	DayTemperatureC                *float64
	NightTemperatureC              *float64
	Forecast                       []WeatherForecastPoint
}

type WeatherForecastPoint struct {
	ObservedAt                  time.Time
	TemperatureC                *float64
	ApparentTemperatureC        *float64
	PrecipitationProbabilityPct *float64
	PrecipitationMm             *float64
	WindSpeedMs                 *float64
	WindGustsMs                 *float64
	WeatherCode                 *int
}

func weatherAdvicePrompt(weather WeatherContext) string {
	var parts []string
	add := func(format string, args ...any) {
		parts = append(parts, fmt.Sprintf(format, args...))
	}

	if strings.TrimSpace(weather.LocationName) != "" {
		add("Город: %s", strings.TrimSpace(weather.LocationName))
	}
	if !weather.ObservedAt.IsZero() {
		add("Время наблюдения: %s", weather.ObservedAt.Format("02.01.2006 15:04"))
	}
	if strings.TrimSpace(weather.Condition) != "" {
		add("Состояние: %s", strings.TrimSpace(weather.Condition))
	}
	add("Температура: %d C", round(weather.TemperatureC))
	if weather.ApparentTemperatureC != nil {
		add("Ощущается как: %d C", round(*weather.ApparentTemperatureC))
	}
	if weather.DayTemperatureC != nil {
		add("Днем: %d C", round(*weather.DayTemperatureC))
	}
	if weather.NightTemperatureC != nil {
		add("Ночью: %d C", round(*weather.NightTemperatureC))
	}
	if weather.WindSpeedMs != nil {
		add(formatWindAdviceLine(weather.WindSpeedMs, weather.WindDirectionDeg))
	}
	if weather.WindGustsMs != nil {
		add("Порывы до %d м/с", round(*weather.WindGustsMs))
	}
	if weather.RelativeHumidityPct != nil {
		add("Влажность %d%%", round(*weather.RelativeHumidityPct))
	}
	if uvLine := formatUVAdviceLine(weather.UVIndexMax, weather.UVIndex); uvLine != "" {
		add(uvLine)
	}
	if weather.PrecipitationProbabilityMaxPct != nil {
		add("Вероятность осадков %d%%", round(*weather.PrecipitationProbabilityMaxPct))
	}
	if weather.PrecipitationMm != nil {
		add("Осадки %.1f мм", *weather.PrecipitationMm)
	}
	if weather.VisibilityM != nil {
		add("Видимость %s км", formatDecimal(*weather.VisibilityM/1000))
	}
	if weather.DewPointC != nil {
		add("Точка росы %d C", round(*weather.DewPointC))
	}
	if weather.SurfacePressureHpa != nil {
		add("Давление %d гПа", round(*weather.SurfacePressureHpa))
	}
	if len(weather.Forecast) > 0 {
		add("Ближайшие часы:")
		for _, point := range weather.Forecast {
			if line := forecastAdviceLine(point); line != "" {
				add("%s", line)
			}
		}
	}

	return "Вот данные погоды на день:\n" +
		strings.Join(parts, "\n") +
		"\n\nДай милый вердикт перед прогулкой и короткий практичный совет по этой погоде на русском языке для печати на чековой ленте. " +
		"Контекст для смысла ответа: у пользователя есть собака-девочка породы джек-рассел, ее зовут Бонни; сейчас они идут гулять. " +
		"Используй имя и породу только как контекст: не упоминай имя Бонни и не упоминай породу без явной необходимости. " +
		"Опирайся только на эти данные. Не выдумывай события, прогнозы, солнце, дождь, снег или сильный ветер, если их нет в данных. " +
		"Совет должен прямо учитывать температуру, осадки, ветер и условие погоды, если они указаны, и не противоречит погоде. " +
		"Не пересказывай уже напечатанный блок погоды: не повторяй цифры, давление и название состояния без необходимости; не начинай с состояния погоды вроде 'Пасмурно' или 'Ясно'. " +
		"Не делай из ответа список обязательных вещей и не навязывай предметы без необходимости. Сформулируй общий совет или мягкое напоминание: как лучше настроиться на прогулку, насколько спокойно идти и на что обратить внимание по погоде. " +
		"Без markdown, без кавычек, 1-2 короткие строки."
}

func round(value float64) int {
	if value < 0 {
		return int(value - 0.5)
	}
	return int(value + 0.5)
}

func formatWindAdviceLine(speedMs *float64, directionDeg *float64) string {
	if speedMs == nil {
		return ""
	}
	direction := weatherdata.WindDirectionLabel(directionDeg)
	if direction == "" {
		return fmt.Sprintf("Ветер %d м/с", round(*speedMs))
	}
	return fmt.Sprintf("%s ветер %d м/с", direction, round(*speedMs))
}

func formatUVAdviceLine(maxUV *float64, currentUV *float64) string {
	if maxUV != nil {
		return fmt.Sprintf("UV сегодня %s %s", formatDecimal(*maxUV), weatherdata.UVIndexLevel(*maxUV))
	}
	if currentUV != nil {
		return fmt.Sprintf("UV сейчас %s %s", formatDecimal(*currentUV), weatherdata.UVIndexLevel(*currentUV))
	}
	return ""
}

func formatDecimal(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%d", round(value))
	}
	return fmt.Sprintf("%.1f", value)
}

func forecastAdviceLine(point WeatherForecastPoint) string {
	if point.ObservedAt.IsZero() {
		return ""
	}
	var parts []string
	if point.TemperatureC != nil {
		parts = append(parts, fmt.Sprintf("%d C", round(*point.TemperatureC)))
	}
	if point.ApparentTemperatureC != nil {
		parts = append(parts, fmt.Sprintf("ощущается %d C", round(*point.ApparentTemperatureC)))
	}
	if point.PrecipitationProbabilityPct != nil {
		parts = append(parts, fmt.Sprintf("осадки %d%%", round(*point.PrecipitationProbabilityPct)))
	}
	if point.PrecipitationMm != nil && *point.PrecipitationMm > 0 {
		parts = append(parts, fmt.Sprintf("дождь %s мм", formatDecimal(*point.PrecipitationMm)))
	}
	if point.WindSpeedMs != nil {
		parts = append(parts, fmt.Sprintf("ветер %d м/с", round(*point.WindSpeedMs)))
	}
	if point.WindGustsMs != nil {
		parts = append(parts, fmt.Sprintf("порывы %d м/с", round(*point.WindGustsMs)))
	}
	if len(parts) == 0 {
		return ""
	}
	return point.ObservedAt.Format("15:04") + ": " + strings.Join(parts, ", ")
}
