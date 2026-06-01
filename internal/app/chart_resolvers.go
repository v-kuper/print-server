package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"atol-server/internal/bankrates"
	"atol-server/internal/chart"
	"atol-server/internal/finance"
	"atol-server/internal/receipt"
)

func (s *ReceiptService) resolveTonChartImage(ctx context.Context, price finance.TonPrice) (*receipt.Image, string) {
	if s.tonChartProvider == nil {
		return nil, ""
	}
	chartData, err := s.tonChartProvider.MarketChart(ctx)
	if err != nil {
		image, fallbackErr := s.renderTonChartImage(fallbackTonMarketChart(price, s.clock()))
		if fallbackErr != nil {
			return nil, "график TON недоступен: " + err.Error()
		}
		return image, "график TON построен по запасным данным: " + err.Error()
	}

	image, err := s.renderTonChartImage(chartData)
	if err != nil {
		return nil, "график TON недоступен: " + err.Error()
	}
	return image, ""
}

func (s *ReceiptService) renderTonChartImage(chartData finance.TonMarketChart) (*receipt.Image, error) {
	path := filepath.Join(s.generatedAssetsPathOrDefault(), "generated", "ton-24h.png")
	chartImage, err := chart.RenderTonPriceChartPixelBuffer(chartData, chart.Options{Width: 384, Height: 96})
	if err != nil {
		return nil, err
	}
	if err := chart.SaveMonoPNG(path, chartImage); err != nil {
		return nil, err
	}

	return &receipt.Image{
		Path:        path,
		URL:         fmt.Sprintf("/assets/generated/ton-24h.png?v=%d", s.clock().UnixNano()),
		Width:       chartImage.Width,
		Height:      chartImage.Height,
		PixelBuffer: chartImage.Pixels,
	}, nil
}

func fallbackTonMarketChart(price finance.TonPrice, now time.Time) finance.TonMarketChart {
	currentPrice := price.USD
	previousPrice := currentPrice
	if price.USD24hChangePercent != nil {
		denominator := 1 + (*price.USD24hChangePercent / 100)
		if denominator > 0.001 {
			previousPrice = currentPrice / denominator
		}
	}
	if now.IsZero() {
		now = time.Now()
	}
	return finance.TonMarketChart{Points: []finance.TonPricePoint{
		{Time: now.Add(-24 * time.Hour), USD: previousPrice},
		{Time: now, USD: currentPrice},
	}}
}

func (s *ReceiptService) resolveUsdBynChartImage(ctx context.Context) (*receipt.Image, string) {
	if s.fiatChartProvider == nil {
		return nil, ""
	}
	chartData, err := s.fiatChartProvider.MarketChart(ctx, s.clock())
	if err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}

	path := filepath.Join(s.generatedAssetsPathOrDefault(), "generated", "usd-byn-7d.png")
	chartImage, err := chart.RenderFiatRateChartPixelBuffer(chartData, chart.Options{Width: 384, Height: 96})
	if err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}
	if err := chart.SaveMonoPNG(path, chartImage); err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}

	return &receipt.Image{
		Path:        path,
		URL:         fmt.Sprintf("/assets/generated/usd-byn-7d.png?v=%d", s.clock().UnixNano()),
		Width:       chartImage.Width,
		Height:      chartImage.Height,
		PixelBuffer: chartImage.Pixels,
	}, ""
}

func (s *ReceiptService) resolveBankRatesSummary(ctx context.Context) (*bankrates.Summary, string) {
	if s.bankRatesProvider == nil {
		return nil, ""
	}
	summary, err := s.bankRatesProvider.Current(ctx)
	if err != nil {
		return nil, "банковские курсы недоступны: " + err.Error()
	}
	return &summary, ""
}

func (s *ReceiptService) generatedAssetsPathOrDefault() string {
	if s.generatedAssetsPath != "" {
		return s.generatedAssetsPath
	}
	return defaultGeneratedAssetsPath
}
