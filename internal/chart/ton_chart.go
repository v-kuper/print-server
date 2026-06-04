package chart

import (
	"fmt"
	"image"
	"image/color"
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

type MonoImage struct {
	Width  int
	Height int
	Pixels []byte
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

func RenderTonPriceChartPixelBuffer(chart finance.TonMarketChart, options Options) (MonoImage, error) {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Time, Value: point.USD})
	}
	return RenderLineChartPixelBuffer(points, options)
}

func RenderFiatRateChart(path string, chart finance.FiatMarketChart, options Options) error {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Date, Value: point.Rate})
	}
	return RenderLineChart(path, points, options)
}

func RenderFiatRateChartPixelBuffer(chart finance.FiatMarketChart, options Options) (MonoImage, error) {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Date, Value: point.Rate})
	}
	return RenderLineChartPixelBuffer(points, options)
}

func RenderOilPriceChart(path string, chart finance.OilMarketChart, options Options) error {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Date, Value: point.ValueUSD})
	}
	return RenderLineChart(path, points, options)
}

func RenderOilPriceChartPixelBuffer(chart finance.OilMarketChart, options Options) (MonoImage, error) {
	points := make([]Point, 0, len(chart.Points))
	for _, point := range chart.Points {
		points = append(points, Point{Time: point.Date, Value: point.ValueUSD})
	}
	return RenderLineChartPixelBuffer(points, options)
}

func RenderLineChart(path string, points []Point, options Options) error {
	chartImage, err := RenderLineChartPixelBuffer(points, options)
	if err != nil {
		return err
	}
	return SaveMonoPNG(path, chartImage)
}

func RenderLineChartPixelBuffer(points []Point, options Options) (MonoImage, error) {
	width := options.Width
	if width <= 0 {
		width = 384
	}
	height := options.Height
	if height <= 0 {
		height = 96
	}
	if len(points) < 2 {
		return MonoImage{}, fmt.Errorf("chart needs at least 2 points")
	}

	canvas := newMonoCanvas(width, height)

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
		return MonoImage{}, fmt.Errorf("chart size is too small: %dx%d", width, height)
	}

	for _, y := range []int{topPad, topPad + plotHeight/2, plotBottom} {
		drawDottedHorizontal(canvas, leftPad, plotRight, y)
	}
	for _, x := range timeTickPositions(leftPad, plotRight, 3) {
		drawDottedVertical(canvas, x, topPad, plotBottom)
		drawThickLine(canvas, image.Point{X: x, Y: plotBottom - 3}, image.Point{X: x, Y: plotBottom + 3}, 1)
	}
	drawThickLine(canvas, image.Point{X: leftPad, Y: plotBottom}, image.Point{X: plotRight, Y: plotBottom}, 1)

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

	for index := 1; index < len(renderedPoints); index++ {
		drawThickLine(canvas, renderedPoints[index-1], renderedPoints[index], 2)
	}
	drawPriceMarker(canvas, renderedPoints[0])
	drawPriceMarker(canvas, renderedPoints[len(renderedPoints)-1])

	return canvas.image(), nil
}

func SaveMonoPNG(path string, chartImage MonoImage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chart directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create chart file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, chartImage.RGBA()); err != nil {
		return fmt.Errorf("encode chart PNG: %w", err)
	}
	return nil
}

func (m MonoImage) RGBA() *image.RGBA {
	imageData := image.NewRGBA(image.Rect(0, 0, m.Width, m.Height))
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			pixel := byte(0)
			index := y*m.Width + x
			if index >= 0 && index < len(m.Pixels) {
				pixel = m.Pixels[index]
			}
			if pixel == 255 {
				imageData.Set(x, y, color.Black)
			} else {
				imageData.Set(x, y, color.White)
			}
		}
	}
	return imageData
}

type monoCanvas struct {
	width  int
	height int
	pixels []byte
}

func newMonoCanvas(width int, height int) *monoCanvas {
	return &monoCanvas{
		width:  width,
		height: height,
		pixels: make([]byte, width*height),
	}
}

func (c *monoCanvas) set(x int, y int) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	c.pixels[y*c.width+x] = 255
}

func (c *monoCanvas) image() MonoImage {
	return MonoImage{
		Width:  c.width,
		Height: c.height,
		Pixels: append([]byte(nil), c.pixels...),
	}
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

func drawDottedHorizontal(target *monoCanvas, fromX int, toX int, y int) {
	for x := fromX; x <= toX; x += 6 {
		for dot := 0; dot < 3 && x+dot <= toX; dot++ {
			target.set(x+dot, y)
		}
	}
}

func drawDottedVertical(target *monoCanvas, x int, fromY int, toY int) {
	for y := fromY; y <= toY; y += 6 {
		for dot := 0; dot < 3 && y+dot <= toY; dot++ {
			target.set(x, y+dot)
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

func drawThickLine(target *monoCanvas, from image.Point, to image.Point, thickness int) {
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
		drawSquare(target, x, y, thickness)
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

func drawSquare(target *monoCanvas, centerX int, centerY int, size int) {
	radius := size / 2
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			target.set(x, y)
		}
	}
}

func drawPriceMarker(target *monoCanvas, point image.Point) {
	drawSquare(target, point.X, point.Y, 4)
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
