package chart

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"atol-server/internal/finance"
)

type Options struct {
	Width  int
	Height int
}

type Point struct {
	Time  time.Time
	Value float64
}

func RenderTonPriceChart(path string, chart finance.TonMarketChart, options Options) error {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Time, Value: point.USD})
	}
	return RenderLineChart(path, points, options)
}

func RenderFiatRateChart(path string, chart finance.FiatMarketChart, options Options) error {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Date, Value: point.Rate})
	}
	return RenderLineChart(path, points, options)
}

func RenderLineChart(path string, points []Point, options Options) error {
	width := options.Width
	if width <= 0 {
		width = 384
	}
	height := options.Height
	if height <= 0 {
		height = 96
	}
	if len(points) < 2 {
		return fmt.Errorf("chart needs at least 2 points")
	}

	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(imageData, imageData.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	minValue, maxValue := valueRange(points)
	if math.Abs(maxValue-minValue) < 0.000001 {
		maxValue += 0.01
		minValue -= 0.01
	}

	const leftPad = 8
	const rightPad = 8
	const topPad = 8
	const bottomPad = 10
	plotRight := width - rightPad - 1
	plotBottom := height - bottomPad - 1
	plotWidth := plotRight - leftPad
	plotHeight := plotBottom - topPad
	if plotWidth <= 0 || plotHeight <= 0 {
		return fmt.Errorf("chart size is too small: %dx%d", width, height)
	}

	grid := color.RGBA{R: 210, G: 210, B: 210, A: 255}
	for _, y := range []int{topPad, topPad + plotHeight/2, plotBottom} {
		drawDottedHorizontal(imageData, leftPad, plotRight, y, grid)
	}
	for _, x := range timeTickPositions(leftPad, plotRight, 3) {
		drawDottedVertical(imageData, x, topPad, plotBottom, grid)
	}

	renderedPoints := make([]image.Point, 0, len(points))
	lastIndex := len(points) - 1
	for index, point := range points {
		x := leftPad
		if lastIndex > 0 {
			x += int(math.Round(float64(index) * float64(plotWidth) / float64(lastIndex)))
		}
		ratio := (point.Value - minValue) / (maxValue - minValue)
		y := topPad + plotHeight - int(math.Round(ratio*float64(plotHeight)))
		renderedPoints = append(renderedPoints, image.Point{X: clamp(x, leftPad, plotRight), Y: clamp(y, topPad, plotBottom)})
	}

	black := color.RGBA{A: 255}
	for index := 1; index < len(renderedPoints); index++ {
		drawThickLine(imageData, renderedPoints[index-1], renderedPoints[index], black, 2)
	}
	drawPriceMarker(imageData, renderedPoints[0], black)
	drawPriceMarker(imageData, renderedPoints[len(renderedPoints)-1], black)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chart directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create chart file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, imageData); err != nil {
		return fmt.Errorf("encode chart PNG: %w", err)
	}
	return nil
}

func valueRange(points []Point) (float64, float64) {
	minValue := points[0].Value
	maxValue := points[0].Value
	for _, point := range points[1:] {
		minValue = math.Min(minValue, point.Value)
		maxValue = math.Max(maxValue, point.Value)
	}
	padding := math.Max((maxValue-minValue)*0.08, 0.002)
	return minValue - padding, maxValue + padding
}

func drawDottedHorizontal(target *image.RGBA, fromX int, toX int, y int, value color.Color) {
	for x := fromX; x <= toX; x += 6 {
		for dot := 0; dot < 3 && x+dot <= toX; dot++ {
			target.Set(x+dot, y, value)
		}
	}
}

func drawDottedVertical(target *image.RGBA, x int, fromY int, toY int, value color.Color) {
	for y := fromY; y <= toY; y += 6 {
		for dot := 0; dot < 3 && y+dot <= toY; dot++ {
			target.Set(x, y+dot, value)
		}
	}
}

func timeTickPositions(left int, right int, count int) []int {
	if count <= 1 {
		return []int{left, right}
	}
	result := make([]int, 0, count)
	for index := 0; index < count; index++ {
		result = append(result, left+int(math.Round(float64(index)*float64(right-left)/float64(count-1))))
	}
	return result
}

func drawThickLine(target *image.RGBA, from image.Point, to image.Point, value color.Color, thickness int) {
	if thickness < 1 {
		thickness = 1
	}
	dx := math.Abs(float64(to.X - from.X))
	dy := -math.Abs(float64(to.Y - from.Y))
	stepX := -1
	if from.X < to.X {
		stepX = 1
	}
	stepY := -1
	if from.Y < to.Y {
		stepY = 1
	}
	err := dx + dy
	x := from.X
	y := from.Y
	for {
		drawSquare(target, x, y, thickness, value)
		if x == to.X && y == to.Y {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += stepX
		}
		if e2 <= dx {
			err += dx
			y += stepY
		}
	}
}

func drawSquare(target *image.RGBA, centerX int, centerY int, size int, value color.Color) {
	radius := size / 2
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if image.Pt(x, y).In(target.Bounds()) {
				target.Set(x, y, value)
			}
		}
	}
}

func drawPriceMarker(target *image.RGBA, point image.Point, value color.Color) {
	drawSquare(target, point.X, point.Y, 4, value)
}

func clamp(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
