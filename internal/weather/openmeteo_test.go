package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpenMeteoProviderParsesCurrentWeather(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Query().Get("latitude") != "52.434500" {
				t.Fatalf("unexpected latitude query: %s", request.URL.RawQuery)
			}
			if request.URL.Query().Get("longitude") != "30.975400" {
				t.Fatalf("unexpected longitude query: %s", request.URL.RawQuery)
			}
			if request.URL.Query().Get("wind_speed_unit") != "ms" {
				t.Fatalf("expected wind speed unit ms, got query: %s", request.URL.RawQuery)
			}
			currentVariables := request.URL.Query().Get("current")
			for _, variable := range []string{"relative_humidity_2m", "rain", "showers", "snowfall", "cloud_cover", "uv_index", "uv_index_clear_sky", "wind_direction_10m", "wind_gusts_10m", "visibility", "dew_point_2m"} {
				if !strings.Contains(currentVariables, variable) {
					t.Fatalf("expected current weather query to include %s, got %q", variable, currentVariables)
				}
			}
			dailyVariables := request.URL.Query().Get("daily")
			for _, variable := range []string{"uv_index_max", "uv_index_clear_sky_max", "precipitation_probability_max", "precipitation_hours", "wind_gusts_10m_max", "sunrise", "sunset", "sunshine_duration"} {
				if !strings.Contains(dailyVariables, variable) {
					t.Fatalf("expected daily weather query to include %s, got %q", variable, dailyVariables)
				}
			}
			hourlyVariables := request.URL.Query().Get("hourly")
			for _, variable := range []string{"temperature_2m", "apparent_temperature", "precipitation_probability", "precipitation", "weather_code", "wind_speed_10m", "wind_gusts_10m"} {
				if !strings.Contains(hourlyVariables, variable) {
					t.Fatalf("expected hourly weather query to include %s, got %q", variable, hourlyVariables)
				}
			}
			if request.URL.Query().Get("forecast_hours") != "6" {
				t.Fatalf("expected 6 forecast hours for AI context, got query: %s", request.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
			"latitude": 52.4345,
			"longitude": 30.9754,
			"timezone": "Europe/Minsk",
			"current": {
				"time": "2026-05-24T09:15",
				"temperature_2m": 21.4,
				"apparent_temperature": 20.8,
				"relative_humidity_2m": 64,
				"dew_point_2m": 12.4,
				"precipitation": 0.2,
				"rain": 0.1,
				"showers": 0.1,
				"snowfall": 0.0,
				"cloud_cover": 92,
				"uv_index": 2.4,
				"uv_index_clear_sky": 4.1,
				"surface_pressure": 1012.4,
				"wind_speed_10m": 11.3,
				"wind_gusts_10m": 15.2,
				"wind_direction_10m": 315,
				"visibility": 12000,
				"weather_code": 61
			},
			"daily": {
				"temperature_2m_max": [24.1],
				"temperature_2m_min": [12.6],
				"uv_index_max": [6.2],
				"uv_index_clear_sky_max": [6.8],
				"precipitation_probability_max": [83],
				"precipitation_hours": [4],
				"wind_gusts_10m_max": [18.4],
				"sunrise": ["2026-05-24T05:03"],
				"sunset": ["2026-05-24T21:08"],
				"sunshine_duration": [14400]
			},
			"hourly": {
				"time": ["2026-05-24T10:00", "2026-05-24T11:00"],
				"temperature_2m": [22.1, 22.4],
				"apparent_temperature": [20.2, 20.6],
				"precipitation_probability": [60, 35],
				"precipitation": [0.3, 0.0],
				"rain": [0.2, 0.0],
				"showers": [0.1, 0.0],
				"snowfall": [0.0, 0.0],
				"weather_code": [61, 3],
				"cloud_cover": [95, 88],
				"wind_speed_10m": [9.1, 8.3],
				"wind_gusts_10m": [14.2, 12.1],
				"wind_direction_10m": [320, 300]
			}
		}`)),
			}, nil
		}),
	}

	provider := NewOpenMeteoProvider(client)
	provider.BaseURL = "https://weather.test/forecast"

	snapshot, err := provider.Current(context.Background(), DefaultLocation())
	if err != nil {
		t.Fatalf("load weather: %v", err)
	}

	if snapshot.LocationName != "Гомель" {
		t.Fatalf("expected location name, got %q", snapshot.LocationName)
	}
	if snapshot.ObservedAt.UTC() != time.Date(2026, 5, 24, 6, 15, 0, 0, time.UTC) {
		t.Fatalf("unexpected observed time: %s", snapshot.ObservedAt.UTC())
	}
	if snapshot.TemperatureC != 21.4 {
		t.Fatalf("expected temperature 21.4, got %v", snapshot.TemperatureC)
	}
	if snapshot.DayTemperatureC == nil || *snapshot.DayTemperatureC != 24.1 {
		t.Fatalf("expected day temperature, got %#v", snapshot.DayTemperatureC)
	}
	if snapshot.NightTemperatureC == nil || *snapshot.NightTemperatureC != 12.6 {
		t.Fatalf("expected night temperature, got %#v", snapshot.NightTemperatureC)
	}
	if snapshot.WindSpeedMs == nil || *snapshot.WindSpeedMs != 11.3 {
		t.Fatalf("expected wind speed in m/s, got %#v", snapshot.WindSpeedMs)
	}
	if snapshot.WindGustsMs == nil || *snapshot.WindGustsMs != 15.2 {
		t.Fatalf("expected wind gusts in m/s, got %#v", snapshot.WindGustsMs)
	}
	if snapshot.WindDirectionDeg == nil || *snapshot.WindDirectionDeg != 315 {
		t.Fatalf("expected wind direction degrees, got %#v", snapshot.WindDirectionDeg)
	}
	if snapshot.DewPointC == nil || *snapshot.DewPointC != 12.4 {
		t.Fatalf("expected dew point, got %#v", snapshot.DewPointC)
	}
	if snapshot.VisibilityM == nil || *snapshot.VisibilityM != 12000 {
		t.Fatalf("expected visibility, got %#v", snapshot.VisibilityM)
	}
	if snapshot.RelativeHumidityPct == nil || *snapshot.RelativeHumidityPct != 64 {
		t.Fatalf("expected relative humidity, got %#v", snapshot.RelativeHumidityPct)
	}
	if snapshot.UVIndex == nil || *snapshot.UVIndex != 2.4 {
		t.Fatalf("expected current UV index, got %#v", snapshot.UVIndex)
	}
	if snapshot.UVIndexClearSky == nil || *snapshot.UVIndexClearSky != 4.1 {
		t.Fatalf("expected clear-sky UV index, got %#v", snapshot.UVIndexClearSky)
	}
	if snapshot.UVIndexMax == nil || *snapshot.UVIndexMax != 6.2 {
		t.Fatalf("expected daily max UV index, got %#v", snapshot.UVIndexMax)
	}
	if snapshot.UVIndexClearSkyMax == nil || *snapshot.UVIndexClearSkyMax != 6.8 {
		t.Fatalf("expected daily max clear-sky UV index, got %#v", snapshot.UVIndexClearSkyMax)
	}
	if snapshot.PrecipitationProbabilityMaxPct == nil || *snapshot.PrecipitationProbabilityMaxPct != 83 {
		t.Fatalf("expected precipitation probability max, got %#v", snapshot.PrecipitationProbabilityMaxPct)
	}
	if snapshot.PrecipitationHours == nil || *snapshot.PrecipitationHours != 4 {
		t.Fatalf("expected precipitation hours, got %#v", snapshot.PrecipitationHours)
	}
	if snapshot.WindGustsMaxMs == nil || *snapshot.WindGustsMaxMs != 18.4 {
		t.Fatalf("expected max wind gusts, got %#v", snapshot.WindGustsMaxMs)
	}
	if snapshot.RainMm == nil || *snapshot.RainMm != 0.1 {
		t.Fatalf("expected rain amount, got %#v", snapshot.RainMm)
	}
	if snapshot.ShowersMm == nil || *snapshot.ShowersMm != 0.1 {
		t.Fatalf("expected showers amount, got %#v", snapshot.ShowersMm)
	}
	if snapshot.SnowfallCm == nil || *snapshot.SnowfallCm != 0 {
		t.Fatalf("expected snowfall amount, got %#v", snapshot.SnowfallCm)
	}
	if snapshot.CloudCoverPct == nil || *snapshot.CloudCoverPct != 92 {
		t.Fatalf("expected cloud cover, got %#v", snapshot.CloudCoverPct)
	}
	if len(snapshot.Forecast) != 2 {
		t.Fatalf("expected two forecast points, got %#v", snapshot.Forecast)
	}
	if snapshot.Forecast[0].PrecipitationProbabilityPct == nil || *snapshot.Forecast[0].PrecipitationProbabilityPct != 60 {
		t.Fatalf("expected first forecast precipitation probability, got %#v", snapshot.Forecast[0])
	}
	if snapshot.Forecast[0].WindGustsMs == nil || *snapshot.Forecast[0].WindGustsMs != 14.2 {
		t.Fatalf("expected first forecast wind gusts, got %#v", snapshot.Forecast[0])
	}
}

func TestNewOpenMeteoProviderUsesWeatherFriendlyTimeout(t *testing.T) {
	provider := NewOpenMeteoProvider(nil)

	if provider.Client.Timeout < 20*time.Second {
		t.Fatalf("expected Open-Meteo timeout to tolerate slow API responses, got %s", provider.Client.Timeout)
	}
}

func TestOpenMeteoProviderRetriesTransientTimeout(t *testing.T) {
	calls := 0
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return nil, context.DeadlineExceeded
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
			"latitude": 52.4345,
			"longitude": 30.9754,
			"timezone": "Europe/Minsk",
			"current": {
				"time": "2026-05-24T09:15",
				"temperature_2m": 21.4
			},
			"daily": {
				"temperature_2m_max": [24.1],
				"temperature_2m_min": [12.6]
			}
		}`)),
			}, nil
		}),
	}

	provider := NewOpenMeteoProvider(client)
	provider.BaseURL = "https://weather.test/forecast"

	if _, err := provider.Current(context.Background(), DefaultLocation()); err != nil {
		t.Fatalf("expected retry to recover weather response, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected one retry after timeout, got %d calls", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestConditionLabelUsesRussianWeatherCodeNames(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "clear", code: 0, want: "Ясно"},
		{name: "cloud", code: 3, want: "Пасмурно"},
		{name: "rain", code: 61, want: "Небольшой дождь"},
		{name: "snow", code: 71, want: "Небольшой снег"},
		{name: "storm", code: 95, want: "Гроза"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConditionLabelForCode(&tt.code, nil, nil); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
