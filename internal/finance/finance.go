package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"
)

const (
	coinGeckoTonURL       = "https://api.coingecko.com/api/v3/simple/price?ids=the-open-network&vs_currencies=usd&include_24hr_change=true"
	coinGeckoTonChartURL  = "https://api.coingecko.com/api/v3/coins/the-open-network/market_chart?vs_currency=usd&days=1"
	nbrbUsdBynURL         = "https://api.nbrb.by/exrates/rates/USD?parammode=2"
	nbrbUsdBynDynamicsURL = "https://api.nbrb.by/exrates/rates/dynamics/431"
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

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
