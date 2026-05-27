package weather

import "time"

type Snapshot struct {
	LocationName                   string
	Latitude                       float64
	Longitude                      float64
	Timezone                       string
	ObservedAt                     time.Time
	TemperatureC                   float64
	ApparentTemperatureC           *float64
	RelativeHumidityPct            *float64
	PrecipitationMm                *float64
	RainMm                         *float64
	ShowersMm                      *float64
	SnowfallCm                     *float64
	CloudCoverPct                  *float64
	WindSpeedMs                    *float64
	WindGustsMs                    *float64
	WindDirectionDeg               *float64
	WeatherCode                    *int
	UVIndex                        *float64
	UVIndexClearSky                *float64
	UVIndexMax                     *float64
	UVIndexClearSkyMax             *float64
	DewPointC                      *float64
	VisibilityM                    *float64
	PrecipitationProbabilityMaxPct *float64
	PrecipitationHours             *float64
	WindGustsMaxMs                 *float64
	SurfacePressureHpa             *float64
	DayTemperatureC                *float64
	NightTemperatureC              *float64
	Forecast                       []ForecastPoint
}

type ForecastPoint struct {
	ObservedAt                  time.Time
	TemperatureC                *float64
	ApparentTemperatureC        *float64
	PrecipitationProbabilityPct *float64
	PrecipitationMm             *float64
	RainMm                      *float64
	ShowersMm                   *float64
	SnowfallCm                  *float64
	CloudCoverPct               *float64
	WindSpeedMs                 *float64
	WindGustsMs                 *float64
	WindDirectionDeg            *float64
	WeatherCode                 *int
}

func (s Snapshot) TimeLocation() *time.Location {
	if s.Timezone == "" {
		return time.Local
	}
	location, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Local
	}
	return location
}
