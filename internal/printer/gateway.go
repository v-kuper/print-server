package printer

import (
	"fmt"
	"path/filepath"
)

const defaultAssetsPath = "/opt/atol-server/assets"

type Gateway struct {
	LibraryPath string
	AssetsPath  string
}

type FontMetric struct {
	Font       int `json:"font"`
	LineLength int `json:"lineLength"`
	FontWidth  int `json:"fontWidth"`
}

func NewGateway(libraryPath string, assetsPath ...string) *Gateway {
	gateway := &Gateway{LibraryPath: libraryPath, AssetsPath: defaultAssetsPath}
	if len(assetsPath) > 0 && assetsPath[0] != "" {
		gateway.AssetsPath = assetsPath[0]
	}
	return gateway
}

func (g *Gateway) weatherIconPath(key string) (string, error) {
	if !isSafeAssetKey(key) {
		return "", fmt.Errorf("invalid weather icon key %q", key)
	}
	assetsPath := g.AssetsPath
	if assetsPath == "" {
		assetsPath = defaultAssetsPath
	}
	return filepath.Join(assetsPath, "weather-icons", "print", key+".png"), nil
}

func (g *Gateway) imagePath(lineKey string, explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}
	return g.weatherIconPath(lineKey)
}

func isSafeAssetKey(key string) bool {
	if key == "" {
		return false
	}
	for _, value := range key {
		switch {
		case value >= 'a' && value <= 'z':
		case value >= '0' && value <= '9':
		case value == '_' || value == '-':
		default:
			return false
		}
	}
	return true
}
