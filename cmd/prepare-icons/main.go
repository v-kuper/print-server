package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"atol-server/internal/iconprep"
)

func main() {
	sourceDir := flag.String("source", filepath.Join("assets", "weather-icons", "source"), "source directory with PNG icons")
	targetDir := flag.String("target", filepath.Join("assets", "weather-icons", "print"), "target directory for prepared PNG icons")
	size := flag.Int("size", iconprep.DefaultSize, "output square size in pixels")
	flag.Parse()

	results, err := iconprep.Prepare(iconprep.Options{
		SourceDir: *sourceDir,
		TargetDir: *targetDir,
		Size:      *size,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		fmt.Printf("%s -> %s\n", result.Source, result.Target)
	}
	fmt.Printf("Prepared %d icon(s) as %dx%d PNG files.\n", len(results), *size, *size)
}
