package iconprep

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareFlattensTransparencyAndResizesToSquare(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeTestPNG(t, filepath.Join(sourceDir, "rain.png"), image.Rect(0, 0, 4, 2))

	results, err := Prepare(Options{
		SourceDir: sourceDir,
		TargetDir: targetDir,
		Size:      8,
	})
	if err != nil {
		t.Fatalf("prepare icons: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one converted icon, got %#v", results)
	}

	output, err := os.Open(filepath.Join(targetDir, "rain.png"))
	if err != nil {
		t.Fatalf("open converted icon: %v", err)
	}
	defer output.Close()
	converted, err := png.Decode(output)
	if err != nil {
		t.Fatalf("decode converted icon: %v", err)
	}

	if converted.Bounds().Dx() != 8 || converted.Bounds().Dy() != 8 {
		t.Fatalf("expected 8x8 output, got %v", converted.Bounds())
	}
	if got := color.RGBAModel.Convert(converted.At(0, 0)).(color.RGBA); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("expected transparent margins to become white, got %#v", got)
	}
	if got := color.RGBAModel.Convert(converted.At(4, 4)).(color.RGBA); got.R > 10 || got.G > 10 || got.B > 10 || got.A != 255 {
		t.Fatalf("expected opaque black source area, got %#v", got)
	}
}

func TestPrepareRejectsEmptySourceDirectory(t *testing.T) {
	_, err := Prepare(Options{
		SourceDir: t.TempDir(),
		TargetDir: t.TempDir(),
		Size:      8,
	})
	if err == nil {
		t.Fatal("expected empty source directory to fail")
	}
}

func writeTestPNG(t *testing.T, path string, bounds image.Rectangle) {
	t.Helper()
	img := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 0})
		}
	}
	img.SetRGBA(1, 0, color.RGBA{0, 0, 0, 255})
	img.SetRGBA(2, 0, color.RGBA{0, 0, 0, 255})
	img.SetRGBA(1, 1, color.RGBA{0, 0, 0, 255})
	img.SetRGBA(2, 1, color.RGBA{0, 0, 0, 255})

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test png: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
}
