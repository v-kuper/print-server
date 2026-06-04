package finance

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	coinGeckoTonURL       = "https://api.coingecko.com/api/v3/simple/price?ids=the-open-network&vs_currencies=usd&include_24hr_change=true"
	coinGeckoTonChartURL  = "https://api.coingecko.com/api/v3/coins/the-open-network/market_chart?vs_currency=usd&days=1"
	nbrbUsdBynURL         = "https://api.nbrb.by/exrates/rates/USD?parammode=2"
	nbrbUsdBynDynamicsURL = "https://api.nbrb.by/exrates/rates/dynamics/431"
	fredBrentOilCSVURL    = "https://fred.stlouisfed.org/graph/fredgraph.csv?id=DCOILBRENTEU"
	dataHubBrentOilCSVURL = "https://datahub.io/core/oil-prices/r/brent-daily.csv"
)

type TonPrice struct {
	USD                 float64
	USD24hChangePercent *float64
}

type TonPricePoint struct {
	Time time.Time
	USD  float64
}

type TonMarketChart struct {
	Points []TonPricePoint
}

type TonPortfolio struct {
	AmountTon   float64 `json:"amountTon"`
	InvestedUSD float64 `json:"investedUsd"`
}

type TonPortfolioSummary struct {
	AmountTon       float64
	Price           TonPrice
	CurrentValueUSD float64
	ProfitLossUSD   float64
}

type FiatRate struct {
	BaseCode  string
	QuoteCode string
	Scale     int
	Rate      float64
}

type FiatRatePoint struct {
	Date time.Time
	Rate float64
}

type FiatMarketChart struct {
	BaseCode  string
	QuoteCode string
	Points    []FiatRatePoint
}

type OilPrice struct {
	Name     string
	Currency string
	Unit     string
	Date     time.Time
	ValueUSD float64
}

type OilPricePoint struct {
	Date     time.Time
	ValueUSD float64
}

type OilMarketChart struct {
	Name     string
	Currency string
	Points   []OilPricePoint
}

func DefaultTonPortfolio() TonPortfolio {
	return TonPortfolio{
		AmountTon:   1230.591,
		InvestedUSD: 1977.58,
	}
}

func (p TonPortfolio) Normalized() TonPortfolio {
	if p.AmountTon == 0 && p.InvestedUSD == 0 {
		return DefaultTonPortfolio()
	}
	return p
}

func (p TonPortfolio) Validate() error {
	if math.IsNaN(p.AmountTon) || math.IsInf(p.AmountTon, 0) || p.AmountTon < 0 {
		return fmt.Errorf("TON amount must be non-negative")
	}
	if math.IsNaN(p.InvestedUSD) || math.IsInf(p.InvestedUSD, 0) || p.InvestedUSD < 0 {
		return fmt.Errorf("invested USD must be non-negative")
	}
	return nil
}

func (p TonPortfolio) ValueAt(price TonPrice) TonPortfolioSummary {
	currentValue := p.AmountTon * price.USD
	return TonPortfolioSummary{
		AmountTon:       p.AmountTon,
		Price:           price,
		CurrentValueUSD: currentValue,
		ProfitLossUSD:   currentValue - p.InvestedUSD,
	}
}

type CoinGeckoTonPriceProvider struct {
	Client   *http.Client
	URL      string
	ChartURL string
}

func NewCoinGeckoTonPriceProvider(client *http.Client) *CoinGeckoTonPriceProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &CoinGeckoTonPriceProvider{
		Client: client,
		URL:    coinGeckoTonURL,
	}
}

func (p *CoinGeckoTonPriceProvider) CurrentPrice(ctx context.Context) (TonPrice, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(), nil)
	if err != nil {
		return TonPrice{}, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := p.client().Do(request)
	if err != nil {
		return TonPrice{}, fmt.Errorf("request CoinGecko: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return TonPrice{}, fmt.Errorf("CoinGecko returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		TheOpenNetwork struct {
			USD       float64  `json:"usd"`
			USDChange *float64 `json:"usd_24h_change"`
		} `json:"the-open-network"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return TonPrice{}, fmt.Errorf("decode CoinGecko response: %w", err)
	}
	if payload.TheOpenNetwork.USD == 0 {
		return TonPrice{}, fmt.Errorf("missing CoinGecko field: usd")
	}

	return TonPrice{
		USD:                 payload.TheOpenNetwork.USD,
		USD24hChangePercent: payload.TheOpenNetwork.USDChange,
	}, nil
}

func (p *CoinGeckoTonPriceProvider) MarketChart(ctx context.Context) (TonMarketChart, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.chartURL(), nil)
	if err != nil {
		return TonMarketChart{}, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := p.client().Do(request)
	if err != nil {
		return TonMarketChart{}, fmt.Errorf("request CoinGecko market chart: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return TonMarketChart{}, fmt.Errorf("CoinGecko market chart returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		Prices [][]float64 `json:"prices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return TonMarketChart{}, fmt.Errorf("decode CoinGecko market chart response: %w", err)
	}

	points := make([]TonPricePoint, 0, len(payload.Prices))
	for _, value := range payload.Prices {
		if len(value) < 2 || value[0] <= 0 || value[1] <= 0 {
			continue
		}
		points = append(points, TonPricePoint{
			Time: time.UnixMilli(int64(value[0])),
			USD:  value[1],
		})
	}
	if len(points) < 2 {
		return TonMarketChart{}, fmt.Errorf("CoinGecko market chart returned less than 2 usable points")
	}

	return TonMarketChart{Points: points}, nil
}

func (p *CoinGeckoTonPriceProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *CoinGeckoTonPriceProvider) url() string {
	if p.URL != "" {
		return p.URL
	}
	return coinGeckoTonURL
}

func (p *CoinGeckoTonPriceProvider) chartURL() string {
	if p.ChartURL != "" {
		return p.ChartURL
	}
	return coinGeckoTonChartURL
}

type NbrbUsdBynRateProvider struct {
	Client      *http.Client
	URL         string
	DynamicsURL string
}

func NewNbrbUsdBynRateProvider(client *http.Client) *NbrbUsdBynRateProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &NbrbUsdBynRateProvider{
		Client: client,
		URL:    nbrbUsdBynURL,
	}
}

func (p *NbrbUsdBynRateProvider) CurrentRate(ctx context.Context) (FiatRate, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(), nil)
	if err != nil {
		return FiatRate{}, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := p.client().Do(request)
	if err != nil {
		return FiatRate{}, fmt.Errorf("request NBRB: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return FiatRate{}, fmt.Errorf("NBRB returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		Abbreviation string  `json:"Cur_Abbreviation"`
		Scale        int     `json:"Cur_Scale"`
		OfficialRate float64 `json:"Cur_OfficialRate"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return FiatRate{}, fmt.Errorf("decode NBRB response: %w", err)
	}
	if payload.OfficialRate == 0 {
		return FiatRate{}, fmt.Errorf("missing NBRB field: Cur_OfficialRate")
	}
	if payload.Abbreviation == "" {
		payload.Abbreviation = "USD"
	}
	if payload.Scale == 0 {
		payload.Scale = 1
	}

	return FiatRate{
		BaseCode:  payload.Abbreviation,
		QuoteCode: "BYN",
		Scale:     payload.Scale,
		Rate:      payload.OfficialRate,
	}, nil
}

func (p *NbrbUsdBynRateProvider) MarketChart(ctx context.Context, end time.Time) (FiatMarketChart, error) {
	if end.IsZero() {
		end = time.Now()
	}
	start := end.AddDate(0, 0, -6)
	requestURL, err := p.marketChartURL(start, end)
	if err != nil {
		return FiatMarketChart{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return FiatMarketChart{}, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := p.client().Do(request)
	if err != nil {
		return FiatMarketChart{}, fmt.Errorf("request NBRB dynamics: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return FiatMarketChart{}, fmt.Errorf("NBRB dynamics returned HTTP %d", response.StatusCode)
	}

	var payload []struct {
		Date         string  `json:"Date"`
		OfficialRate float64 `json:"Cur_OfficialRate"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return FiatMarketChart{}, fmt.Errorf("decode NBRB dynamics response: %w", err)
	}

	points := make([]FiatRatePoint, 0, len(payload))
	for _, value := range payload {
		if value.OfficialRate <= 0 {
			continue
		}
		date, err := parseNbrbDate(value.Date)
		if err != nil {
			continue
		}
		points = append(points, FiatRatePoint{
			Date: date,
			Rate: value.OfficialRate,
		})
	}
	if len(points) < 2 {
		return FiatMarketChart{}, fmt.Errorf("NBRB dynamics returned less than 2 usable points")
	}

	return FiatMarketChart{
		BaseCode:  "USD",
		QuoteCode: "BYN",
		Points:    points,
	}, nil
}

func (p *NbrbUsdBynRateProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *NbrbUsdBynRateProvider) url() string {
	if p.URL != "" {
		return p.URL
	}
	return nbrbUsdBynURL
}

func (p *NbrbUsdBynRateProvider) dynamicsURL() string {
	if p.DynamicsURL != "" {
		return p.DynamicsURL
	}
	return nbrbUsdBynDynamicsURL
}

func (p *NbrbUsdBynRateProvider) marketChartURL(start time.Time, end time.Time) (string, error) {
	parsed, err := url.Parse(p.dynamicsURL())
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("startdate", start.Format("2006-01-02"))
	query.Set("enddate", end.Format("2006-01-02"))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func parseNbrbDate(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported NBRB date %q", value)
}

type FredBrentOilPriceProvider struct {
	Client *http.Client
	URL    string
}

type BrentOilPriceProvider interface {
	CurrentPrice(context.Context) (OilPrice, error)
	MarketChart(context.Context) (OilMarketChart, error)
}

func NewFredBrentOilPriceProvider(client *http.Client) *FredBrentOilPriceProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FredBrentOilPriceProvider{
		Client: client,
		URL:    fredBrentOilCSVURL,
	}
}

func NewDefaultBrentOilPriceProvider() *FallbackBrentOilPriceProvider {
	return NewFallbackBrentOilPriceProvider(
		NewFredBrentOilPriceProvider(&http.Client{Timeout: 4 * time.Second}),
		NewDataHubBrentOilPriceProvider(&http.Client{Timeout: 10 * time.Second}),
	)
}

func (p *FredBrentOilPriceProvider) CurrentPrice(ctx context.Context) (OilPrice, error) {
	points, err := p.loadPoints(ctx)
	if err != nil {
		return OilPrice{}, err
	}
	return oilPriceFromPoints(points, "FRED Brent CSV")
}

func (p *FredBrentOilPriceProvider) MarketChart(ctx context.Context) (OilMarketChart, error) {
	points, err := p.loadPoints(ctx)
	if err != nil {
		return OilMarketChart{}, err
	}
	return oilChartFromPoints(points, "FRED Brent CSV")
}

func (p *FredBrentOilPriceProvider) loadPoints(ctx context.Context) ([]OilPricePoint, error) {
	return loadOilCSVPoints(ctx, p.client(), p.url(), "FRED Brent CSV")
}

func (p *FredBrentOilPriceProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *FredBrentOilPriceProvider) url() string {
	if p.URL != "" {
		return p.URL
	}
	return fredBrentOilCSVURL
}

type DataHubBrentOilPriceProvider struct {
	Client *http.Client
	URL    string
}

func NewDataHubBrentOilPriceProvider(client *http.Client) *DataHubBrentOilPriceProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &DataHubBrentOilPriceProvider{
		Client: client,
		URL:    dataHubBrentOilCSVURL,
	}
}

func (p *DataHubBrentOilPriceProvider) CurrentPrice(ctx context.Context) (OilPrice, error) {
	points, err := p.loadPoints(ctx)
	if err != nil {
		return OilPrice{}, err
	}
	return oilPriceFromPoints(points, "DataHub Brent CSV")
}

func (p *DataHubBrentOilPriceProvider) MarketChart(ctx context.Context) (OilMarketChart, error) {
	points, err := p.loadPoints(ctx)
	if err != nil {
		return OilMarketChart{}, err
	}
	return oilChartFromPoints(points, "DataHub Brent CSV")
}

func (p *DataHubBrentOilPriceProvider) loadPoints(ctx context.Context) ([]OilPricePoint, error) {
	return loadOilCSVPoints(ctx, p.client(), p.url(), "DataHub Brent CSV")
}

func (p *DataHubBrentOilPriceProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *DataHubBrentOilPriceProvider) url() string {
	if p.URL != "" {
		return p.URL
	}
	return dataHubBrentOilCSVURL
}

type FallbackBrentOilPriceProvider struct {
	Primary           BrentOilPriceProvider
	Fallback          BrentOilPriceProvider
	PrimaryRetryAfter time.Duration

	mu             sync.Mutex
	primaryRetryAt time.Time
}

func NewFallbackBrentOilPriceProvider(primary BrentOilPriceProvider, fallback BrentOilPriceProvider) *FallbackBrentOilPriceProvider {
	return &FallbackBrentOilPriceProvider{
		Primary:  primary,
		Fallback: fallback,
	}
}

func (p *FallbackBrentOilPriceProvider) CurrentPrice(ctx context.Context) (OilPrice, error) {
	if p.shouldUsePrimary() {
		price, err := p.Primary.CurrentPrice(ctx)
		if err == nil {
			return price, nil
		}
		p.markPrimaryFailed()
		if p.Fallback == nil {
			return OilPrice{}, err
		}
		fallbackPrice, fallbackErr := p.Fallback.CurrentPrice(ctx)
		if fallbackErr == nil {
			return fallbackPrice, nil
		}
		return OilPrice{}, fmt.Errorf("primary Brent oil provider failed: %v; fallback Brent oil provider failed: %w", err, fallbackErr)
	}
	if p.Fallback == nil {
		return OilPrice{}, fmt.Errorf("missing Brent oil provider")
	}
	return p.Fallback.CurrentPrice(ctx)
}

func (p *FallbackBrentOilPriceProvider) MarketChart(ctx context.Context) (OilMarketChart, error) {
	if p.shouldUsePrimary() {
		chart, err := p.Primary.MarketChart(ctx)
		if err == nil {
			return chart, nil
		}
		p.markPrimaryFailed()
		if p.Fallback == nil {
			return OilMarketChart{}, err
		}
		fallbackChart, fallbackErr := p.Fallback.MarketChart(ctx)
		if fallbackErr == nil {
			return fallbackChart, nil
		}
		return OilMarketChart{}, fmt.Errorf("primary Brent oil provider failed: %v; fallback Brent oil provider failed: %w", err, fallbackErr)
	}
	if p.Fallback == nil {
		return OilMarketChart{}, fmt.Errorf("missing Brent oil provider")
	}
	return p.Fallback.MarketChart(ctx)
}

func (p *FallbackBrentOilPriceProvider) shouldUsePrimary() bool {
	if p.Primary == nil {
		return false
	}
	if p.Fallback == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.primaryRetryAt.IsZero() || !time.Now().Before(p.primaryRetryAt)
}

func (p *FallbackBrentOilPriceProvider) markPrimaryFailed() {
	if p.Fallback == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	retryAfter := p.PrimaryRetryAfter
	if retryAfter <= 0 {
		retryAfter = 30 * time.Minute
	}
	p.primaryRetryAt = time.Now().Add(retryAfter)
}

func loadOilCSVPoints(ctx context.Context, client *http.Client, rawURL string, sourceName string) ([]OilPricePoint, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/csv")

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", sourceName, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("%s returned HTTP %d", sourceName, response.StatusCode)
	}

	reader := csv.NewReader(response.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", sourceName, err)
	}
	return parseOilCSVRecords(records), nil
}

func parseOilCSVRecords(records [][]string) []OilPricePoint {
	if len(records) == 0 {
		return nil
	}
	dateIndex, valueIndex := oilCSVColumnIndexes(records[0])
	points := make([]OilPricePoint, 0, len(records)-1)
	for _, record := range records[1:] {
		if len(record) <= dateIndex || len(record) <= valueIndex {
			continue
		}
		date, err := time.Parse("2006-01-02", strings.TrimSpace(record[dateIndex]))
		if err != nil {
			continue
		}
		rawValue := strings.TrimSpace(record[valueIndex])
		if rawValue == "" || rawValue == "." {
			continue
		}
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil || value <= 0 {
			continue
		}
		points = append(points, OilPricePoint{Date: date, ValueUSD: value})
	}
	return points
}

func oilCSVColumnIndexes(header []string) (int, int) {
	dateIndex := 0
	valueIndex := 1
	for index, column := range header {
		normalized := strings.ToLower(strings.TrimSpace(column))
		switch normalized {
		case "date", "observation_date":
			dateIndex = index
		case "price", "dcoilbrenteu":
			valueIndex = index
		}
	}
	return dateIndex, valueIndex
}

func oilPriceFromPoints(points []OilPricePoint, sourceName string) (OilPrice, error) {
	if len(points) == 0 {
		return OilPrice{}, fmt.Errorf("%s returned no usable oil prices", sourceName)
	}
	latest := points[len(points)-1]
	return OilPrice{
		Name:     "Brent",
		Currency: "USD",
		Unit:     "barrel",
		Date:     latest.Date,
		ValueUSD: latest.ValueUSD,
	}, nil
}

func oilChartFromPoints(points []OilPricePoint, sourceName string) (OilMarketChart, error) {
	if len(points) > 7 {
		points = points[len(points)-7:]
	}
	if len(points) < 2 {
		return OilMarketChart{}, fmt.Errorf("%s returned less than 2 usable points", sourceName)
	}
	return OilMarketChart{
		Name:     "Brent",
		Currency: "USD",
		Points:   points,
	}, nil
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
