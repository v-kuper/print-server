package weather

import "testing"

func TestConditionTextUsesDetailedWeatherIcons(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "clear", code: 0, want: "☀ Ясно"},
		{name: "mainly clear", code: 1, want: "☀ Малооблачно"},
		{name: "partly cloudy", code: 2, want: "⛅ Переменная облачность"},
		{name: "overcast", code: 3, want: "☁ Пасмурно"},
		{name: "fog", code: 45, want: "≋ Туман"},
		{name: "drizzle", code: 53, want: "☂ Морось"},
		{name: "freezing drizzle", code: 57, want: "☂❄ Ледяная морось"},
		{name: "rain", code: 63, want: "☔ Дождь"},
		{name: "freezing rain", code: 67, want: "☔❄ Ледяной дождь"},
		{name: "snow", code: 73, want: "❄ Снег"},
		{name: "snow grains", code: 77, want: "❄ Снежная крупа"},
		{name: "rain showers", code: 81, want: "☔ Ливень"},
		{name: "snow showers", code: 85, want: "❄ Снежный заряд"},
		{name: "thunderstorm", code: 95, want: "⚡ Гроза"},
		{name: "hail thunderstorm", code: 99, want: "⚡ Гроза с градом"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConditionTextForCode(&tt.code, nil, nil); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestConditionTextFallsBackToWindAndPrecipitationIcons(t *testing.T) {
	wind := 11.0
	precipitation := 0.4

	if got := ConditionTextForCode(nil, nil, &wind); got != "↗ Ветрено" {
		t.Fatalf("expected strong wind fallback, got %q", got)
	}
	if got := ConditionTextForCode(nil, &precipitation, nil); got != "☔ Дождь" {
		t.Fatalf("expected precipitation fallback, got %q", got)
	}
}

func TestConditionIconKeyUsesPrintableWeatherAssets(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "clear", code: 0, want: "clear"},
		{name: "partly cloudy", code: 2, want: "partly_cloudy"},
		{name: "overcast", code: 3, want: "cloudy"},
		{name: "fog", code: 45, want: "fog"},
		{name: "drizzle", code: 53, want: "drizzle"},
		{name: "rain", code: 63, want: "rain"},
		{name: "heavy rain", code: 65, want: "heavy_rain"},
		{name: "freezing rain", code: 67, want: "freezing_rain"},
		{name: "snow", code: 73, want: "snow"},
		{name: "snow showers", code: 85, want: "snow_showers"},
		{name: "thunderstorm", code: 95, want: "thunderstorm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConditionIconKeyForCode(&tt.code, nil, nil); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestConditionIconKeyFallsBackToWindAndPrecipitation(t *testing.T) {
	wind := 11.0
	precipitation := 0.4

	if got := ConditionIconKeyForCode(nil, nil, &wind); got != "wind" {
		t.Fatalf("expected wind fallback, got %q", got)
	}
	if got := ConditionIconKeyForCode(nil, &precipitation, nil); got != "rain" {
		t.Fatalf("expected rain fallback, got %q", got)
	}
}

func TestConditionDisplayTreatsDryHailCodeAsCloudCover(t *testing.T) {
	code := 99
	precipitation := 0.0
	rain := 0.0
	showers := 0.0
	snowfall := 0.0
	cloudCover := 94.0

	got := ConditionDisplayForSnapshot(Snapshot{
		WeatherCode:     &code,
		PrecipitationMm: &precipitation,
		RainMm:          &rain,
		ShowersMm:       &showers,
		SnowfallCm:      &snowfall,
		CloudCoverPct:   &cloudCover,
	})

	if got.IconKey != "cloudy" || got.Label != "Пасмурно" {
		t.Fatalf("expected dry hail code to fall back to overcast, got %#v", got)
	}
}

func TestPrecipitationIconUsesWeatherCodeFamily(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "rain", code: 61, want: "☔"},
		{name: "snow", code: 71, want: "❄"},
		{name: "freezing rain", code: 66, want: "☔❄"},
		{name: "storm", code: 95, want: "⚡"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrecipitationIconForCode(&tt.code); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestWindDirectionLabelUsesRussianCompassPoints(t *testing.T) {
	tests := []struct {
		degrees float64
		want    string
	}{
		{degrees: 0, want: "Северный"},
		{degrees: 90, want: "Восточный"},
		{degrees: 225, want: "Юго-западный"},
		{degrees: 315, want: "Северо-западный"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := WindDirectionLabel(&tt.degrees); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
