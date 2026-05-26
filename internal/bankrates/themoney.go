package bankrates

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	theMoneyUsdBynURL = "https://themoney.by/dollar-exchange-rate/"
	defaultSourceName = "TheMoney.by"
	defaultCityName   = "Минск"
)

type Offer struct {
	Rate      float64
	BankNames []string
}

type Summary struct {
	Source    string
	City      string
	SellUSD   *Offer
	BuyUSD    *Offer
	UpdatedAt time.Time
}

type TheMoneyProvider struct {
	Client *http.Client
	URL    string
}

func NewTheMoneyProvider(client *http.Client) *TheMoneyProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &TheMoneyProvider{
		Client: client,
		URL:    theMoneyUsdBynURL,
	}
}

func (p *TheMoneyProvider) Current(ctx context.Context) (Summary, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(), nil)
	if err != nil {
		return Summary{}, err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "atol-receipt-server/1.0")

	response, err := p.client().Do(request)
	if err != nil {
		return Summary{}, fmt.Errorf("request TheMoney: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Summary{}, fmt.Errorf("TheMoney returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return Summary{}, fmt.Errorf("read TheMoney response: %w", err)
	}
	return ParseTheMoneySummary(body)
}

func ParseTheMoneySummary(body []byte) (Summary, error) {
	payload := normalizePayload(string(body))
	bankBuyRates, buyErr := extractRateEntries(payload, "buyData")
	bankSellRates, sellErr := extractRateEntries(payload, "sellData")
	if buyErr != nil && sellErr != nil {
		return Summary{}, fmt.Errorf("parse TheMoney rates: %v; %v", buyErr, sellErr)
	}

	sellUSD := bestMaxRate(bankBuyRates)
	buyUSD := bestMinRate(bankSellRates)
	if sellUSD == nil && buyUSD == nil {
		return Summary{}, fmt.Errorf("TheMoney response contains no usable USD/BYN rates")
	}

	return Summary{
		Source:    defaultSourceName,
		City:      defaultCityName,
		SellUSD:   sellUSD,
		BuyUSD:    buyUSD,
		UpdatedAt: latestUpdate(bankBuyRates, bankSellRates),
	}, nil
}

func normalizePayload(value string) string {
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, `\"`, `"`)
	return value
}

type rateEntry struct {
	BankName    string
	Rate        float64
	CollectedAt time.Time
}

func extractRateEntries(payload string, key string) ([]rateEntry, error) {
	keyIndex := strings.Index(payload, `"`+key+`"`)
	if keyIndex < 0 {
		return nil, fmt.Errorf("missing %s", key)
	}
	ratesIndex := strings.Index(payload[keyIndex:], `"rates":[`)
	if ratesIndex < 0 {
		return nil, fmt.Errorf("missing %s.rates", key)
	}
	arrayStart := keyIndex + ratesIndex + len(`"rates":`)
	arrayEnd := matchingJSONEnd(payload, arrayStart)
	if arrayEnd < 0 {
		return nil, fmt.Errorf("invalid %s.rates JSON array", key)
	}

	var rawRates []struct {
		Bank struct {
			Title string `json:"title"`
		} `json:"bank"`
		Rate        float64 `json:"rate"`
		CollectedAt string  `json:"collectedAt"`
	}
	if err := json.Unmarshal([]byte(payload[arrayStart:arrayEnd]), &rawRates); err != nil {
		return nil, fmt.Errorf("decode %s.rates: %w", key, err)
	}

	result := make([]rateEntry, 0, len(rawRates))
	for _, rawRate := range rawRates {
		if rawRate.Rate <= 0 || strings.TrimSpace(rawRate.Bank.Title) == "" {
			continue
		}
		entry := rateEntry{
			BankName: strings.TrimSpace(rawRate.Bank.Title),
			Rate:     rawRate.Rate,
		}
		if rawRate.CollectedAt != "" {
			if collectedAt, err := time.Parse(time.RFC3339Nano, rawRate.CollectedAt); err == nil {
				entry.CollectedAt = collectedAt
			}
		}
		result = append(result, entry)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s.rates contains no usable rates", key)
	}
	return result, nil
}

func matchingJSONEnd(payload string, start int) int {
	if start < 0 || start >= len(payload) || payload[start] != '[' {
		return -1
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(payload); index++ {
		current := payload[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch current {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch current {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index + 1
			}
		}
	}
	return -1
}

func bestMaxRate(entries []rateEntry) *Offer {
	if len(entries) == 0 {
		return nil
	}
	best := entries[0].Rate
	var bankNames []string
	for _, entry := range entries {
		switch {
		case sameRate(entry.Rate, best):
			bankNames = append(bankNames, entry.BankName)
		case entry.Rate > best:
			best = entry.Rate
			bankNames = []string{entry.BankName}
		}
	}
	return &Offer{Rate: best, BankNames: uniqueBankNames(bankNames)}
}

func bestMinRate(entries []rateEntry) *Offer {
	if len(entries) == 0 {
		return nil
	}
	best := entries[0].Rate
	var bankNames []string
	for _, entry := range entries {
		switch {
		case sameRate(entry.Rate, best):
			bankNames = append(bankNames, entry.BankName)
		case entry.Rate < best:
			best = entry.Rate
			bankNames = []string{entry.BankName}
		}
	}
	return &Offer{Rate: best, BankNames: uniqueBankNames(bankNames)}
}

func uniqueBankNames(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sameRate(left float64, right float64) bool {
	return math.Abs(left-right) < 0.0000001
}

func latestUpdate(groups ...[]rateEntry) time.Time {
	var latest time.Time
	for _, group := range groups {
		for _, entry := range group {
			if entry.CollectedAt.After(latest) {
				latest = entry.CollectedAt
			}
		}
	}
	return latest
}

func (p *TheMoneyProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *TheMoneyProvider) url() string {
	if p.URL != "" {
		return p.URL
	}
	return theMoneyUsdBynURL
}
