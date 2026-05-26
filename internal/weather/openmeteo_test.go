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
				"precipitation": 0.2,
				"surface_pressure": 1012.4,
				"wind_speed_10m": 11.3,
				"weather_code": 61
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
