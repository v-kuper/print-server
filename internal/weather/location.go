package weather

import (
	"fmt"
	"math"
	"strings"
)

type Location struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func DefaultLocation() Location {
	return Location{
		Name:      "Гомель",
		Latitude:  52.4345,
		Longitude: 30.9754,
	}
}

func (l Location) Normalized() Location {
	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		l.Name = DefaultLocation().Name
	}
	return l
}

func (l Location) Validate() error {
	if math.IsNaN(l.Latitude) || math.IsInf(l.Latitude, 0) || l.Latitude < -90 || l.Latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if math.IsNaN(l.Longitude) || math.IsInf(l.Longitude, 0) || l.Longitude < -180 || l.Longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}
