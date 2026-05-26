package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGeocodingProviderSearchesOpenMeteoLocations(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			query := request.URL.Query()
			if query.Get("name") != "Гомель" {
				t.Fatalf("unexpected name query: %s", request.URL.RawQuery)
			}
			if query.Get("count") != "8" {
				t.Fatalf("unexpected count query: %s", request.URL.RawQuery)
			}
			if query.Get("language") != "ru" {
				t.Fatalf("unexpected language query: %s", request.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"results": [
						{
							"id": 627907,
							"name": "Гомель",
							"latitude": 52.4345,
							"longitude": 30.9754,
							"country": "Беларусь",
							"admin1": "Гомельская область",
							"timezone": "Europe/Minsk",
							"population": 526872
						}
					]
				}`)),
			}, nil
		}),
	}
	provider := NewGeocodingProvider(client)
	provider.BaseURL = "https://geocoding.test/search"

	results, err := provider.Search(context.Background(), " Гомель ")
	if err != nil {
		t.Fatalf("search locations: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	if results[0].Name != "Гомель" ||
		results[0].Latitude != 52.4345 ||
		results[0].Longitude != 30.9754 ||
		results[0].DisplayName != "Гомель, Гомельская область, Беларусь" {
		t.Fatalf("unexpected geocoding result: %#v", results[0])
	}
}

func TestGeocodingProviderReturnsEmptyForShortQuery(t *testing.T) {
	provider := NewGeocodingProvider(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("short query must not call geocoding API")
			return nil, nil
		}),
	})

	results, err := provider.Search(context.Background(), "г")
	if err != nil {
		t.Fatalf("search short query: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results for short query, got %#v", results)
	}
}
