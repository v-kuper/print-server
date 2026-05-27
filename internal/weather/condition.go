package weather

import "math"

const strongWindMs = 10.0

type ConditionInfo struct {
	Icon    string
	IconKey string
	Label   string
}

type conditionInput struct {
	code            *int
	precipitationMm *float64
	rainMm          *float64
	showersMm       *float64
	snowfallCm      *float64
	cloudCoverPct   *float64
	windSpeedMs     *float64
}

func ConditionLabel(snapshot Snapshot) string {
	return ConditionDisplayForSnapshot(snapshot).Label
}

func ConditionLabelForCode(code *int, precipitationMm *float64, windSpeedMs *float64) string {
	return ConditionDisplayForCode(code, precipitationMm, windSpeedMs).Label
}

func ConditionText(snapshot Snapshot) string {
	return ConditionDisplay(snapshot).Text()
}

func ConditionTextForCode(code *int, precipitationMm *float64, windSpeedMs *float64) string {
	return ConditionDisplayForCode(code, precipitationMm, windSpeedMs).Text()
}

func ConditionDisplay(snapshot Snapshot) ConditionInfo {
	return ConditionDisplayForSnapshot(snapshot)
}

func ConditionDisplayForSnapshot(snapshot Snapshot) ConditionInfo {
	return conditionDisplay(conditionInput{
		code:            snapshot.WeatherCode,
		precipitationMm: snapshot.PrecipitationMm,
		rainMm:          snapshot.RainMm,
		showersMm:       snapshot.ShowersMm,
		snowfallCm:      snapshot.SnowfallCm,
		cloudCoverPct:   snapshot.CloudCoverPct,
		windSpeedMs:     snapshot.WindSpeedMs,
	})
}

func ConditionDisplayForCode(code *int, precipitationMm *float64, windSpeedMs *float64) ConditionInfo {
	return conditionDisplay(conditionInput{
		code:            code,
		precipitationMm: precipitationMm,
		windSpeedMs:     windSpeedMs,
	})
}

func conditionDisplay(input conditionInput) ConditionInfo {
	display := ConditionInfo{Icon: "☁", IconKey: "cloudy", Label: "Облачно"}
	if input.code != nil {
		switch *input.code {
		case 0:
			display = ConditionInfo{Icon: "☀", IconKey: "clear", Label: "Ясно"}
		case 1:
			display = ConditionInfo{Icon: "☀", IconKey: "partly_cloudy", Label: "Малооблачно"}
		case 2:
			display = ConditionInfo{Icon: "⛅", IconKey: "partly_cloudy", Label: "Переменная облачность"}
		case 3:
			display = ConditionInfo{Icon: "☁", IconKey: "cloudy", Label: "Пасмурно"}
		case 45:
			display = ConditionInfo{Icon: "≋", IconKey: "fog", Label: "Туман"}
		case 48:
			display = ConditionInfo{Icon: "≋", IconKey: "fog", Label: "Инейный туман"}
		case 51:
			display = ConditionInfo{Icon: "☂", IconKey: "drizzle", Label: "Легкая морось"}
		case 53:
			display = ConditionInfo{Icon: "☂", IconKey: "drizzle", Label: "Морось"}
		case 55:
			display = ConditionInfo{Icon: "☂", IconKey: "drizzle", Label: "Сильная морось"}
		case 56, 57:
			display = ConditionInfo{Icon: "☂❄", IconKey: "freezing_rain", Label: "Ледяная морось"}
		case 61:
			display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Небольшой дождь"}
		case 63:
			display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Дождь"}
		case 65:
			display = ConditionInfo{Icon: "☔", IconKey: "heavy_rain", Label: "Сильный дождь"}
		case 66, 67:
			display = ConditionInfo{Icon: "☔❄", IconKey: "freezing_rain", Label: "Ледяной дождь"}
		case 71:
			display = ConditionInfo{Icon: "❄", IconKey: "snow", Label: "Небольшой снег"}
		case 73:
			display = ConditionInfo{Icon: "❄", IconKey: "snow", Label: "Снег"}
		case 75:
			display = ConditionInfo{Icon: "❄", IconKey: "snow", Label: "Сильный снег"}
		case 77:
			display = ConditionInfo{Icon: "❄", IconKey: "snow", Label: "Снежная крупа"}
		case 80:
			display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Небольшой ливень"}
		case 81:
			display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Ливень"}
		case 82:
			display = ConditionInfo{Icon: "☔", IconKey: "heavy_rain", Label: "Сильный ливень"}
		case 85:
			display = ConditionInfo{Icon: "❄", IconKey: "snow_showers", Label: "Снежный заряд"}
		case 86:
			display = ConditionInfo{Icon: "❄", IconKey: "snow_showers", Label: "Сильный снежный заряд"}
		case 95:
			display = ConditionInfo{Icon: "⚡", IconKey: "thunderstorm", Label: "Гроза"}
		case 96, 99:
			display = ConditionInfo{Icon: "⚡", IconKey: "thunderstorm", Label: "Гроза с градом"}
		default:
			if input.precipitationMm != nil && *input.precipitationMm > 0 {
				display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Дождь"}
			}
		}
	} else if input.precipitationMm != nil && *input.precipitationMm > 0 {
		display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Дождь"}
	}

	if isDryHailCode(input) {
		display = cloudCoverDisplay(input.cloudCoverPct)
	}

	if input.windSpeedMs != nil && *input.windSpeedMs >= strongWindMs {
		switch display.Label {
		case "Ясно", "Малооблачно", "Переменная облачность", "Пасмурно", "Облачно":
			return ConditionInfo{Icon: "↗", IconKey: "wind", Label: "Ветрено"}
		}
	}
	return display
}

func isDryHailCode(input conditionInput) bool {
	if input.code == nil || (*input.code != 96 && *input.code != 99) || input.cloudCoverPct == nil {
		return false
	}
	hasAmount := false
	total := 0.0
	for _, value := range []*float64{input.precipitationMm, input.rainMm, input.showersMm, input.snowfallCm} {
		if value == nil {
			continue
		}
		hasAmount = true
		total += *value
	}
	return hasAmount && total <= 0
}

func cloudCoverDisplay(cloudCoverPct *float64) ConditionInfo {
	if cloudCoverPct == nil {
		return ConditionInfo{Icon: "☁", IconKey: "cloudy", Label: "Облачно"}
	}
	switch {
	case *cloudCoverPct >= 85:
		return ConditionInfo{Icon: "☁", IconKey: "cloudy", Label: "Пасмурно"}
	case *cloudCoverPct >= 50:
		return ConditionInfo{Icon: "⛅", IconKey: "partly_cloudy", Label: "Переменная облачность"}
	case *cloudCoverPct >= 15:
		return ConditionInfo{Icon: "☀", IconKey: "partly_cloudy", Label: "Малооблачно"}
	default:
		return ConditionInfo{Icon: "☀", IconKey: "clear", Label: "Ясно"}
	}
}

func ConditionIconKey(snapshot Snapshot) string {
	return ConditionDisplay(snapshot).IconKey
}

func ConditionIconKeyForCode(code *int, precipitationMm *float64, windSpeedMs *float64) string {
	return ConditionDisplayForCode(code, precipitationMm, windSpeedMs).IconKey
}

func (d ConditionInfo) Text() string {
	if d.Icon == "" {
		return d.Label
	}
	return d.Icon + " " + d.Label
}

func PrecipitationIcon(snapshot Snapshot) string {
	return PrecipitationIconForCode(snapshot.WeatherCode)
}

func PrecipitationIconForCode(code *int) string {
	if code == nil {
		return "☔"
	}

	switch *code {
	case 56, 57, 66, 67:
		return "☔❄"
	case 71, 73, 75, 77, 85, 86:
		return "❄"
	case 95, 96, 99:
		return "⚡"
	default:
		return "☔"
	}
}

func WindDirectionLabel(degrees *float64) string {
	if degrees == nil || math.IsNaN(*degrees) || math.IsInf(*degrees, 0) {
		return ""
	}
	normalized := math.Mod(*degrees, 360)
	if normalized < 0 {
		normalized += 360
	}
	labels := []string{
		"Северный",
		"Северо-восточный",
		"Восточный",
		"Юго-восточный",
		"Южный",
		"Юго-западный",
		"Западный",
		"Северо-западный",
	}
	index := int(math.Floor((normalized+22.5)/45.0)) % len(labels)
	return labels[index]
}

func UVIndexLevel(value float64) string {
	switch {
	case value < 3:
		return "низкий"
	case value < 6:
		return "умеренный"
	case value < 8:
		return "высокий"
	case value < 11:
		return "очень высокий"
	default:
		return "экстремальный"
	}
}
