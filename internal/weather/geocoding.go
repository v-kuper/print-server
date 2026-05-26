package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultGeocodingURL = "https://geocoding-api.open-meteo.com/v1/search"

type LocationCandidate struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Country     string  `json:"country,omitempty"`
	Admin1      string  `json:"admin1,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	Population  int     `json:"population,omitempty"`
	DisplayName string  `json:"displayName"`
}

type GeocodingProvider struct {
	Client  *http.Client
	BaseURL string
	Count   int
}

func NewGeocodingProvider(client *http.Client) *GeocodingProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &GeocodingProvider{
		Client:  client,
		BaseURL: defaultGeocodingURL,
		Count:   8,
	}
}

func (p *GeocodingProvider) Search(ctx context.Context, query string) ([]LocationCandidate, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.buildURL(query), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ATOL-Go-Server/1.0")

	response, err := p.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Open-Meteo geocoding: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("Open-Meteo geocoding returned HTTP %d", response.StatusCode)
	}

	var payload geocodingResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Open-Meteo geocoding response: %w", err)
	}

	results := make([]LocationCandidate, 0, len(payload.Results))
	for _, result := range payload.Results {
		result.Name = strings.TrimSpace(result.Name)
		result.Country = strings.TrimSpace(result.Country)
		result.Admin1 = strings.TrimSpace(result.Admin1)
		result.Timezone = strings.TrimSpace(result.Timezone)
		result.DisplayName = locationDisplayName(result)
		results = append(results, result)
	}
	return results, nil
}

func (p *GeocodingProvider) buildURL(query string) string {
	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = defaultGeocodingURL
	}
	count := p.Count
	if count <= 0 || count > 100 {
		count = 8
	}

	values := url.Values{}
	values.Set("name", query)
	values.Set("count", strconv.Itoa(count))
	values.Set("language", "ru")
	values.Set("format", "json")
	return baseURL + "?" + values.Encode()
}

func (p *GeocodingProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

type geocodingResponse struct {
	Results []LocationCandidate `json:"results"`
}

func locationDisplayName(candidate LocationCandidate) string {
	parts := []string{candidate.Name}
	if candidate.Admin1 != "" && candidate.Admin1 != candidate.Name {
		parts = append(parts, candidate.Admin1)
	}
	if candidate.Country != "" {
		parts = append(parts, candidate.Country)
	}
	return strings.Join(parts, ", ")
}
