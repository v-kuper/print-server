package chart

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"atol-server/internal/finance"
)

func TestRenderTonPriceChartWritesPrintablePNG(t *testing.T) {
	target := filepath.Join(t.TempDir(), "ton-24h.png")
	chartData := finance.TonMarketChart{Points: []finance.TonPricePoint{
		{Time: time.Unix(0, 0), USD: 1.74},
		{Time: time.Unix(3600, 0), USD: 1.80},
		{Time: time.Unix(7200, 0), USD: 1.72},
		{Time: time.Unix(10800, 0), USD: 1.77},
	}}

	if err := RenderTonPriceChart(target, chartData, Options{Width: 384, Height: 96}); err != nil {
		t.Fatalf("render chart: %v", err)
	}

	file, err := os.Open(target)
	if err != nil {
		t.Fatalf("open rendered chart: %v", err)
	}
	defer file.Close()

	image, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode rendered chart: %v", err)
	}
	bounds := image.Bounds()
	if bounds.Dx() != 384 || bounds.Dy() != 96 {
		t.Fatalf("expected 384x96 chart, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	darkPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if isDark(image.At(x, y)) {
				darkPixels++
			}
		}
	}
	if darkPixels < 200 {
		t.Fatalf("expected visible chart line and labels, got %d dark pixels", darkPixels)
	}

	if !hasVerticalTick(image, 8) || !hasVerticalTick(image, bounds.Dx()/2) || !hasVerticalTick(image, bounds.Dx()-9) {
		t.Fatalf("expected visible vertical time ticks")
	}
}

func TestRenderFiatRateChartWritesPrintablePNG(t *testing.T) {
	target := filepath.Join(t.TempDir(), "usd-byn-7d.png")
	chartData := finance.FiatMarketChart{
		BaseCode:  "USD",
		QuoteCode: "BYN",
		Points: []finance.FiatRatePoint{
			{Date: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC), Rate: 3.10},
			{Date: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), Rate: 3.12},
			{Date: time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC), Rate: 3.11},
			{Date: time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC), Rate: 3.13},
		},
	}

	if err := RenderFiatRateChart(target, chartData, Options{Width: 384, Height: 96}); err != nil {
		t.Fatalf("render chart: %v", err)
	}

	file, err := os.Open(target)
	if err != nil {
		t.Fatalf("open rendered chart: %v", err)
	}
	defer file.Close()

	image, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode rendered chart: %v", err)
	}
	bounds := image.Bounds()
	if bounds.Dx() != 384 || bounds.Dy() != 96 {
		t.Fatalf("expected 384x96 chart, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	if !hasVerticalTick(image, 8) || !hasVerticalTick(image, bounds.Dx()/2) || !hasVerticalTick(image, bounds.Dx()-9) {
		t.Fatalf("expected visible vertical time ticks")
	}
}

func isDark(value color.Color) bool {
	r, g, b, _ := value.RGBA()
	return r < 0x8000 && g < 0x8000 && b < 0x8000
}

func hasVerticalTick(imageData interface {
	Bounds() image.Rectangle
	At(x int, y int) color.Color
}, x int) bool {
	bounds := imageData.Bounds()
	nonWhitePixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		if !isWhite(imageData.At(x, y)) {
			nonWhitePixels++
		}
	}
	return nonWhitePixels >= 8
}

func isWhite(value color.Color) bool {
	r, g, b, _ := value.RGBA()
	return r > 0xf000 && g > 0xf000 && b > 0xf000
}
