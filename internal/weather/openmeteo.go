package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultOpenMeteoURL = "https://api.open-meteo.com/v1/forecast"
const defaultOpenMeteoTimeout = 25 * time.Second

type OpenMeteoProvider struct {
	Client  *http.Client
	BaseURL string
}

func NewOpenMeteoProvider(client *http.Client) *OpenMeteoProvider {
	if client == nil {
		client = &http.Client{Timeout: defaultOpenMeteoTimeout}
	}
	return &OpenMeteoProvider{
		Client:  client,
		BaseURL: defaultOpenMeteoURL,
	}
}

func (p *OpenMeteoProvider) Current(ctx context.Context, location Location) (Snapshot, error) {
	normalized := location.Normalized()
	if err := normalized.Validate(); err != nil {
		return Snapshot{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.buildURL(normalized), nil)
	if err != nil {
		return Snapshot{}, err
	}

	response, err := p.doRequest(ctx, request)
	if err != nil {
		return Snapshot{}, fmt.Errorf("request Open-Meteo: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Snapshot{}, fmt.Errorf("Open-Meteo returned HTTP %d", response.StatusCode)
	}

	var payload openMeteoResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Snapshot{}, fmt.Errorf("decode Open-Meteo response: %w", err)
	}
	return payload.snapshot(normalized.Name)
}

func (p *OpenMeteoProvider) buildURL(location Location) string {
	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenMeteoURL
	}

	values := url.Values{}
	values.Set("latitude", strconv.FormatFloat(location.Latitude, 'f', 6, 64))
	values.Set("longitude", strconv.FormatFloat(location.Longitude, 'f', 6, 64))
	values.Set("current", "temperature_2m,apparent_temperature,precipitation,surface_pressure,wind_speed_10m,weather_code")
	values.Set("daily", "temperature_2m_max,temperature_2m_min")
	values.Set("forecast_days", "1")
	values.Set("timezone", "auto")
	values.Set("wind_speed_unit", "ms")
	return baseURL + "?" + values.Encode()
}

func (p *OpenMeteoProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: defaultOpenMeteoTimeout}
}

func (p *OpenMeteoProvider) doRequest(ctx context.Context, request *http.Request) (*http.Response, error) {
	response, err := p.client().Do(request)
	if err == nil || !isRetryableWeatherRequestError(ctx, err) {
		return response, err
	}
	retryRequest := request.Clone(ctx)
	return p.client().Do(retryRequest)
}

func isRetryableWeatherRequestError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

type openMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Current   struct {
		Time                string   `json:"time"`
		TemperatureC        float64  `json:"temperature_2m"`
		ApparentTemperature *float64 `json:"apparent_temperature"`
		Precipitation       *float64 `json:"precipitation"`
		SurfacePressure     *float64 `json:"surface_pressure"`
		WindSpeed           *float64 `json:"wind_speed_10m"`
		WeatherCode         *int     `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		TemperatureMax []float64 `json:"temperature_2m_max"`
		TemperatureMin []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

func (r openMeteoResponse) snapshot(locationName string) (Snapshot, error) {
	timezone := r.Timezone
	location := time.Local
	if timezone != "" {
		loaded, err := time.LoadLocation(timezone)
		if err != nil {
			return Snapshot{}, fmt.Errorf("load Open-Meteo timezone %q: %w", timezone, err)
		}
		location = loaded
	}

	observedAt, err := parseOpenMeteoTime(r.Current.Time, location)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		LocationName:         locationName,
		Latitude:             r.Latitude,
		Longitude:            r.Longitude,
		Timezone:             timezone,
		ObservedAt:           observedAt,
		TemperatureC:         r.Current.TemperatureC,
		ApparentTemperatureC: r.Current.ApparentTemperature,
		PrecipitationMm:      r.Current.Precipitation,
		WindSpeedMs:          r.Current.WindSpeed,
		WeatherCode:          r.Current.WeatherCode,
		SurfacePressureHpa:   r.Current.SurfacePressure,
		DayTemperatureC:      firstFloat64(r.Daily.TemperatureMax),
		NightTemperatureC:    firstFloat64(r.Daily.TemperatureMin),
	}, nil
}

func parseOpenMeteoTime(value string, location *time.Location) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("missing Open-Meteo current time")
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, location)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse Open-Meteo current time %q", value)
}

func firstFloat64(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}
