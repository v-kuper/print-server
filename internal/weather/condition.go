package weather

const strongWindMs = 10.0

type ConditionInfo struct {
	Icon    string
	IconKey string
	Label   string
}

func ConditionLabel(snapshot Snapshot) string {
	return ConditionDisplayForCode(snapshot.WeatherCode, snapshot.PrecipitationMm, snapshot.WindSpeedMs).Label
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
	return ConditionDisplayForCode(snapshot.WeatherCode, snapshot.PrecipitationMm, snapshot.WindSpeedMs)
}

func ConditionDisplayForCode(code *int, precipitationMm *float64, windSpeedMs *float64) ConditionInfo {
	display := ConditionInfo{Icon: "☁", IconKey: "cloudy", Label: "Облачно"}
	if code != nil {
		switch *code {
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
			if precipitationMm != nil && *precipitationMm > 0 {
				display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Дождь"}
			}
		}
	} else if precipitationMm != nil && *precipitationMm > 0 {
		display = ConditionInfo{Icon: "☔", IconKey: "rain", Label: "Дождь"}
	}

	if windSpeedMs != nil && *windSpeedMs >= strongWindMs {
		switch display.Label {
		case "Ясно", "Малооблачно", "Переменная облачность", "Пасмурно", "Облачно":
			return ConditionInfo{Icon: "↗", IconKey: "wind", Label: "Ветрено"}
		}
	}
	return display
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
