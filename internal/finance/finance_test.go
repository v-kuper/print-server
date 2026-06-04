package finance

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCoinGeckoProviderParsesTonUsdPrice(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("ids") != "the-open-network" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		return jsonResponse(`{
			"the-open-network": {
				"usd": 1.7435687405,
				"usd_24h_change": -1.15
			}
		}`), nil
	})}
	provider := NewCoinGeckoTonPriceProvider(client)

	price, err := provider.CurrentPrice(context.Background())
	if err != nil {
		t.Fatalf("load TON price: %v", err)
	}

	if price.USD != 1.7435687405 {
		t.Fatalf("expected USD price, got %v", price.USD)
	}
	if price.USD24hChangePercent == nil || *price.USD24hChangePercent != -1.15 {
		t.Fatalf("expected day change, got %#v", price.USD24hChangePercent)
	}
}

func TestCoinGeckoProviderParsesTonUsdMarketChart(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v3/coins/the-open-network/market_chart" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("vs_currency") != "usd" || request.URL.Query().Get("days") != "1" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		return jsonResponse(`{
			"prices": [
				[1779631200000, 1.741],
				[1779634800000, 1.755],
				[1779638400000, 1.732]
			]
		}`), nil
	})}
	provider := NewCoinGeckoTonPriceProvider(client)

	chart, err := provider.MarketChart(context.Background())
	if err != nil {
		t.Fatalf("load TON market chart: %v", err)
	}

	if len(chart.Points) != 3 {
		t.Fatalf("expected 3 chart points, got %#v", chart.Points)
	}
	if !chart.Points[0].Time.Equal(time.UnixMilli(1779631200000)) {
		t.Fatalf("unexpected first timestamp: %s", chart.Points[0].Time)
	}
	if math.Abs(chart.Points[1].USD-1.755) > 0.000001 {
		t.Fatalf("unexpected second price: %v", chart.Points[1].USD)
	}
}

func TestNbrbProviderParsesUsdBynRate(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("parammode") != "2" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		return jsonResponse(`{
			"Cur_Abbreviation": "USD",
			"Cur_Scale": 1,
			"Cur_OfficialRate": 3.1234
		}`), nil
	})}
	provider := NewNbrbUsdBynRateProvider(client)

	rate, err := provider.CurrentRate(context.Background())
	if err != nil {
		t.Fatalf("load USD/BYN rate: %v", err)
	}

	if rate.BaseCode != "USD" || rate.QuoteCode != "BYN" {
		t.Fatalf("unexpected pair: %#v", rate)
	}
	if rate.Scale != 1 {
		t.Fatalf("expected scale 1, got %d", rate.Scale)
	}
	if rate.Rate != 3.1234 {
		t.Fatalf("expected rate 3.1234, got %v", rate.Rate)
	}
}

func TestNbrbProviderParsesUsdBynWeeklyMarketChart(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/exrates/rates/dynamics/431" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("startdate") != "2026-05-19" || request.URL.Query().Get("enddate") != "2026-05-25" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		return jsonResponse(`[
			{"Cur_ID": 431, "Date": "2026-05-19T00:00:00", "Cur_OfficialRate": 3.1012},
			{"Cur_ID": 431, "Date": "2026-05-20T00:00:00", "Cur_OfficialRate": 3.1144},
			{"Cur_ID": 431, "Date": "2026-05-21T00:00:00", "Cur_OfficialRate": 3.1078}
		]`), nil
	})}
	provider := NewNbrbUsdBynRateProvider(client)

	chart, err := provider.MarketChart(context.Background(), time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load USD/BYN market chart: %v", err)
	}

	if chart.BaseCode != "USD" || chart.QuoteCode != "BYN" {
		t.Fatalf("unexpected chart pair: %#v", chart)
	}
	if len(chart.Points) != 3 {
		t.Fatalf("expected 3 chart points, got %#v", chart.Points)
	}
	if chart.Points[0].Date.Format("2006-01-02") != "2026-05-19" {
		t.Fatalf("unexpected first date: %s", chart.Points[0].Date)
	}
	if math.Abs(chart.Points[2].Rate-3.1078) > 0.000001 {
		t.Fatalf("unexpected last rate: %v", chart.Points[2].Rate)
	}
}

func TestFredProviderParsesLatestBrentOilPrice(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/graph/fredgraph.csv" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("id") != "DCOILBRENTEU" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		return csvResponse(`observation_date,DCOILBRENTEU
2026-05-27,97.11
2026-05-28,95.47
2026-05-29,.
2026-06-01,98.29
`), nil
	})}
	provider := NewFredBrentOilPriceProvider(client)

	price, err := provider.CurrentPrice(context.Background())
	if err != nil {
		t.Fatalf("load Brent oil price: %v", err)
	}

	if price.Name != "Brent" {
		t.Fatalf("expected Brent name, got %q", price.Name)
	}
	if price.Currency != "USD" {
		t.Fatalf("expected USD currency, got %q", price.Currency)
	}
	if price.Unit != "barrel" {
		t.Fatalf("expected barrel unit, got %q", price.Unit)
	}
	if price.ValueUSD != 98.29 {
		t.Fatalf("expected latest value 98.29, got %v", price.ValueUSD)
	}
	if price.Date.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("unexpected latest date: %s", price.Date)
	}
}

func TestFredProviderParsesBrentOilWeeklyMarketChart(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return csvResponse(`observation_date,DCOILBRENTEU
2026-05-19,91.01
2026-05-20,92.02
2026-05-21,.
2026-05-22,93.03
2026-05-26,102.75
2026-05-27,97.11
2026-05-28,95.47
2026-05-29,92.88
2026-06-01,98.29
`), nil
	})}
	provider := NewFredBrentOilPriceProvider(client)

	chart, err := provider.MarketChart(context.Background())
	if err != nil {
		t.Fatalf("load Brent oil market chart: %v", err)
	}

	if chart.Name != "Brent" || chart.Currency != "USD" {
		t.Fatalf("unexpected chart metadata: %#v", chart)
	}
	if len(chart.Points) != 7 {
		t.Fatalf("expected last 7 valid points, got %#v", chart.Points)
	}
	if chart.Points[0].Date.Format("2006-01-02") != "2026-05-20" {
		t.Fatalf("unexpected first retained date: %s", chart.Points[0].Date)
	}
	if math.Abs(chart.Points[6].ValueUSD-98.29) > 0.000001 {
		t.Fatalf("unexpected last price: %v", chart.Points[6].ValueUSD)
	}
}

func TestFallbackOilProviderUsesMirrorWhenPrimaryFails(t *testing.T) {
	primaryClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, errors.New("timeout")
	})}
	mirrorClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/core/oil-prices/r/brent-daily.csv" {
			t.Fatalf("unexpected mirror path: %s", request.URL.Path)
		}
		return csvResponse(`Date,Price
2026-05-27,97.11
2026-05-28,95.47
2026-05-29,92.88
2026-06-01,98.29
`), nil
	})}
	provider := NewFallbackBrentOilPriceProvider(
		NewFredBrentOilPriceProvider(primaryClient),
		NewDataHubBrentOilPriceProvider(mirrorClient),
	)

	price, err := provider.CurrentPrice(context.Background())
	if err != nil {
		t.Fatalf("load Brent oil price: %v", err)
	}
	if price.ValueUSD != 98.29 {
		t.Fatalf("expected mirror price 98.29, got %v", price.ValueUSD)
	}

	chart, err := provider.MarketChart(context.Background())
	if err != nil {
		t.Fatalf("load Brent oil market chart: %v", err)
	}
	if len(chart.Points) != 4 {
		t.Fatalf("expected mirror chart points, got %#v", chart.Points)
	}
	if chart.Points[0].Date.Format("2006-01-02") != "2026-05-27" {
		t.Fatalf("unexpected first mirror point: %s", chart.Points[0].Date)
	}
}

func TestFallbackOilProviderSkipsPrimaryChartAfterPrimaryPriceFails(t *testing.T) {
	primary := &fakeBrentOilProvider{priceErr: errors.New("timeout")}
	fallback := &fakeBrentOilProvider{
		price: financeTestOilPrice(98.29),
		chart: financeTestOilChart([]float64{
			97.11,
			95.47,
			92.88,
			98.29,
		}),
	}
	provider := NewFallbackBrentOilPriceProvider(primary, fallback)

	if _, err := provider.CurrentPrice(context.Background()); err != nil {
		t.Fatalf("load Brent oil price: %v", err)
	}
	if _, err := provider.MarketChart(context.Background()); err != nil {
		t.Fatalf("load Brent oil market chart: %v", err)
	}
	if primary.chartCalls != 0 {
		t.Fatalf("expected primary chart to be skipped after primary price failure, got %d calls", primary.chartCalls)
	}
	if fallback.chartCalls != 1 {
		t.Fatalf("expected fallback chart to be called once, got %d", fallback.chartCalls)
	}
}

func TestTonPortfolioCalculatesProfitLoss(t *testing.T) {
	price := TonPrice{USD: 1.7435687405482407}
	summary := DefaultTonPortfolio().ValueAt(price)

	if summary.AmountTon != 1230.591 {
		t.Fatalf("expected default TON amount, got %v", summary.AmountTon)
	}
	if roundMoney(summary.CurrentValueUSD) != 2145.62 {
		t.Fatalf("expected current value 2145.62, got %v", summary.CurrentValueUSD)
	}
	if roundMoney(summary.ProfitLossUSD) != 168.04 {
		t.Fatalf("expected profit 168.04, got %v", summary.ProfitLossUSD)
	}
}

type fakeBrentOilProvider struct {
	price      OilPrice
	priceErr   error
	chart      OilMarketChart
	chartErr   error
	priceCalls int
	chartCalls int
}

func (p *fakeBrentOilProvider) CurrentPrice(context.Context) (OilPrice, error) {
	p.priceCalls++
	if p.priceErr != nil {
		return OilPrice{}, p.priceErr
	}
	return p.price, nil
}

func (p *fakeBrentOilProvider) MarketChart(context.Context) (OilMarketChart, error) {
	p.chartCalls++
	if p.chartErr != nil {
		return OilMarketChart{}, p.chartErr
	}
	return p.chart, nil
}

func financeTestOilPrice(value float64) OilPrice {
	return OilPrice{
		Name:     "Brent",
		Currency: "USD",
		Unit:     "barrel",
		Date:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		ValueUSD: value,
	}
}

func financeTestOilChart(values []float64) OilMarketChart {
	points := make([]OilPricePoint, 0, len(values))
	for index, value := range values {
		points = append(points, OilPricePoint{
			Date:     time.Date(2026, 5, 27+index, 0, 0, 0, 0, time.UTC),
			ValueUSD: value,
		})
	}
	return OilMarketChart{
		Name:     "Brent",
		Currency: "USD",
		Points:   points,
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func csvResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/csv"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
