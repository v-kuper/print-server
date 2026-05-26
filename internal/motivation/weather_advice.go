package motivation

import (
	"fmt"
	"strings"
	"time"
)

type WeatherAdvice struct {
	Text string `json:"text"`
}

type WeatherContext struct {
	LocationName         string
	ObservedAt           time.Time
	Condition            string
	TemperatureC         float64
	ApparentTemperatureC *float64
	PrecipitationMm      *float64
	WindSpeedMs          *float64
	SurfacePressureHpa   *float64
	DayTemperatureC      *float64
	NightTemperatureC    *float64
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
		add("Ветер %d м/с", round(*weather.WindSpeedMs))
	}
	if weather.PrecipitationMm != nil {
		add("Осадки %.1f мм", *weather.PrecipitationMm)
	}
	if weather.SurfacePressureHpa != nil {
		add("Давление %d гПа", round(*weather.SurfacePressureHpa))
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
