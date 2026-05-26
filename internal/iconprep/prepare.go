package iconprep

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultSize = 96

type Options struct {
	SourceDir string
	TargetDir string
	Size      int
}

type Result struct {
	Source string
	Target string
}

func Prepare(options Options) ([]Result, error) {
	sourceDir := strings.TrimSpace(options.SourceDir)
	if sourceDir == "" {
		return nil, fmt.Errorf("source directory is required")
	}
	targetDir := strings.TrimSpace(options.TargetDir)
	if targetDir == "" {
		return nil, fmt.Errorf("target directory is required")
	}
	size := options.Size
	if size <= 0 {
		size = DefaultSize
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read source directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no PNG icons found in %s", sourceDir)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create target directory: %w", err)
	}

	results := make([]Result, 0, len(names))
	for _, name := range names {
		sourcePath := filepath.Join(sourceDir, name)
		targetPath := filepath.Join(targetDir, name)
		if err := prepareIcon(sourcePath, targetPath, size); err != nil {
			return nil, err
		}
		results = append(results, Result{Source: sourcePath, Target: targetPath})
	}
	return results, nil
}

func prepareIcon(sourcePath string, targetPath string, size int) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer input.Close()

	source, err := png.Decode(input)
	if err != nil {
		return fmt.Errorf("decode %s: %w", sourcePath, err)
	}

	target := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(target, target.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	destination := fitRect(source.Bounds(), target.Bounds())
	drawScaledNearest(target, source, destination)

	output, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", targetPath, err)
	}
	defer output.Close()
	if err := png.Encode(output, target); err != nil {
		return fmt.Errorf("encode %s: %w", targetPath, err)
	}
	return nil
}

func fitRect(source image.Rectangle, target image.Rectangle) image.Rectangle {
	sourceWidth := source.Dx()
	sourceHeight := source.Dy()
	targetWidth := target.Dx()
	targetHeight := target.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 || targetWidth <= 0 || targetHeight <= 0 {
		return target
	}

	width := targetWidth
	height := sourceHeight * width / sourceWidth
	if height > targetHeight {
		height = targetHeight
		width = sourceWidth * height / sourceHeight
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	x := target.Min.X + (targetWidth-width)/2
	y := target.Min.Y + (targetHeight-height)/2
	return image.Rect(x, y, x+width, y+height)
}

func drawScaledNearest(target draw.Image, source image.Image, destination image.Rectangle) {
	sourceBounds := source.Bounds()
	for y := destination.Min.Y; y < destination.Max.Y; y++ {
		sourceY := sourceBounds.Min.Y + (y-destination.Min.Y)*sourceBounds.Dy()/destination.Dy()
		for x := destination.Min.X; x < destination.Max.X; x++ {
			sourceX := sourceBounds.Min.X + (x-destination.Min.X)*sourceBounds.Dx()/destination.Dx()
			target.Set(x, y, flattenOnWhite(source.At(sourceX, sourceY)))
		}
	}
}

func flattenOnWhite(value color.Color) color.Color {
	red, green, blue, alpha := value.RGBA()
	if alpha == 0 {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	if alpha == 0xffff {
		return color.RGBA{R: uint8(red >> 8), G: uint8(green >> 8), B: uint8(blue >> 8), A: 255}
	}

	const max = 0xffff
	r := (red*alpha + max*(max-alpha)) / max
	g := (green*alpha + max*(max-alpha)) / max
	b := (blue*alpha + max*(max-alpha)) / max
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
}
