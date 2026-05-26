package weather

import "time"

type Snapshot struct {
	LocationName         string
	Latitude             float64
	Longitude            float64
	Timezone             string
	ObservedAt           time.Time
	TemperatureC         float64
	ApparentTemperatureC *float64
	PrecipitationMm      *float64
	WindSpeedMs          *float64
	WeatherCode          *int
	SurfacePressureHpa   *float64
	DayTemperatureC      *float64
	NightTemperatureC    *float64
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
