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
	values.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,dew_point_2m,precipitation,rain,showers,snowfall,cloud_cover,uv_index,uv_index_clear_sky,surface_pressure,wind_speed_10m,wind_gusts_10m,wind_direction_10m,visibility,weather_code")
	values.Set("hourly", "temperature_2m,apparent_temperature,precipitation_probability,precipitation,rain,showers,snowfall,cloud_cover,weather_code,wind_speed_10m,wind_gusts_10m,wind_direction_10m")
	values.Set("daily", "temperature_2m_max,temperature_2m_min,uv_index_max,uv_index_clear_sky_max,precipitation_probability_max,precipitation_hours,wind_gusts_10m_max,sunrise,sunset,sunshine_duration")
	values.Set("forecast_hours", "6")
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
		RelativeHumidity    *float64 `json:"relative_humidity_2m"`
		DewPoint            *float64 `json:"dew_point_2m"`
		Precipitation       *float64 `json:"precipitation"`
		Rain                *float64 `json:"rain"`
		Showers             *float64 `json:"showers"`
		Snowfall            *float64 `json:"snowfall"`
		CloudCover          *float64 `json:"cloud_cover"`
		UVIndex             *float64 `json:"uv_index"`
		UVIndexClearSky     *float64 `json:"uv_index_clear_sky"`
		SurfacePressure     *float64 `json:"surface_pressure"`
		WindSpeed           *float64 `json:"wind_speed_10m"`
		WindGusts           *float64 `json:"wind_gusts_10m"`
		WindDirection       *float64 `json:"wind_direction_10m"`
		Visibility          *float64 `json:"visibility"`
		WeatherCode         *int     `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		TemperatureMax              []float64 `json:"temperature_2m_max"`
		TemperatureMin              []float64 `json:"temperature_2m_min"`
		UVIndexMax                  []float64 `json:"uv_index_max"`
		UVIndexClearSkyMax          []float64 `json:"uv_index_clear_sky_max"`
		PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max"`
		PrecipitationHours          []float64 `json:"precipitation_hours"`
		WindGustsMax                []float64 `json:"wind_gusts_10m_max"`
	} `json:"daily"`
	Hourly struct {
		Time                        []string   `json:"time"`
		TemperatureC                []*float64 `json:"temperature_2m"`
		ApparentTemperature         []*float64 `json:"apparent_temperature"`
		PrecipitationProbabilityPct []*float64 `json:"precipitation_probability"`
		Precipitation               []*float64 `json:"precipitation"`
		Rain                        []*float64 `json:"rain"`
		Showers                     []*float64 `json:"showers"`
		Snowfall                    []*float64 `json:"snowfall"`
		CloudCover                  []*float64 `json:"cloud_cover"`
		WeatherCode                 []*int     `json:"weather_code"`
		WindSpeed                   []*float64 `json:"wind_speed_10m"`
		WindGusts                   []*float64 `json:"wind_gusts_10m"`
		WindDirection               []*float64 `json:"wind_direction_10m"`
	} `json:"hourly"`
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
		LocationName:                   locationName,
		Latitude:                       r.Latitude,
		Longitude:                      r.Longitude,
		Timezone:                       timezone,
		ObservedAt:                     observedAt,
		TemperatureC:                   r.Current.TemperatureC,
		ApparentTemperatureC:           r.Current.ApparentTemperature,
		RelativeHumidityPct:            r.Current.RelativeHumidity,
		DewPointC:                      r.Current.DewPoint,
		PrecipitationMm:                r.Current.Precipitation,
		RainMm:                         r.Current.Rain,
		ShowersMm:                      r.Current.Showers,
		SnowfallCm:                     r.Current.Snowfall,
		CloudCoverPct:                  r.Current.CloudCover,
		WindSpeedMs:                    r.Current.WindSpeed,
		WindGustsMs:                    r.Current.WindGusts,
		WindDirectionDeg:               r.Current.WindDirection,
		VisibilityM:                    r.Current.Visibility,
		WeatherCode:                    r.Current.WeatherCode,
		UVIndex:                        r.Current.UVIndex,
		UVIndexClearSky:                r.Current.UVIndexClearSky,
		SurfacePressureHpa:             r.Current.SurfacePressure,
		DayTemperatureC:                firstFloat64(r.Daily.TemperatureMax),
		NightTemperatureC:              firstFloat64(r.Daily.TemperatureMin),
		UVIndexMax:                     firstFloat64(r.Daily.UVIndexMax),
		UVIndexClearSkyMax:             firstFloat64(r.Daily.UVIndexClearSkyMax),
		PrecipitationProbabilityMaxPct: firstFloat64(r.Daily.PrecipitationProbabilityMax),
		PrecipitationHours:             firstFloat64(r.Daily.PrecipitationHours),
		WindGustsMaxMs:                 firstFloat64(r.Daily.WindGustsMax),
		Forecast:                       r.forecast(location),
	}, nil
}

func (r openMeteoResponse) forecast(location *time.Location) []ForecastPoint {
	points := make([]ForecastPoint, 0, len(r.Hourly.Time))
	for index, value := range r.Hourly.Time {
		observedAt, err := parseOpenMeteoTime(value, location)
		if err != nil {
			continue
		}
		points = append(points, ForecastPoint{
			ObservedAt:                  observedAt,
			TemperatureC:                floatAt(r.Hourly.TemperatureC, index),
			ApparentTemperatureC:        floatAt(r.Hourly.ApparentTemperature, index),
			PrecipitationProbabilityPct: floatAt(r.Hourly.PrecipitationProbabilityPct, index),
			PrecipitationMm:             floatAt(r.Hourly.Precipitation, index),
			RainMm:                      floatAt(r.Hourly.Rain, index),
			ShowersMm:                   floatAt(r.Hourly.Showers, index),
			SnowfallCm:                  floatAt(r.Hourly.Snowfall, index),
			CloudCoverPct:               floatAt(r.Hourly.CloudCover, index),
			WindSpeedMs:                 floatAt(r.Hourly.WindSpeed, index),
			WindGustsMs:                 floatAt(r.Hourly.WindGusts, index),
			WindDirectionDeg:            floatAt(r.Hourly.WindDirection, index),
			WeatherCode:                 intAt(r.Hourly.WeatherCode, index),
		})
	}
	return points
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

func floatAt(values []*float64, index int) *float64 {
	if index < 0 || index >= len(values) {
		return nil
	}
	return values[index]
}

func intAt(values []*int, index int) *int {
	if index < 0 || index >= len(values) {
		return nil
	}
	return values[index]
}
