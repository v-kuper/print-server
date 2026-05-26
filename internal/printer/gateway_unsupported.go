//go:build !linux

package printer

import (
	"context"
	"fmt"

	"atol-server/internal/receipt"
)

func (g *Gateway) DriverVersion() (string, error) {
	return "", unsupportedError()
}

func (g *Gateway) CheckConnection(_ context.Context, _ Config) (string, error) {
	return "", unsupportedError()
}

func (g *Gateway) PrintReceipt(_ context.Context, _ Config, _ []receipt.Line) error {
	return unsupportedError()
}

func (g *Gateway) FontMetrics(_ context.Context, _ Config) ([]FontMetric, error) {
	return nil, unsupportedError()
}

func unsupportedError() error {
	return fmt.Errorf("ATOL native driver is available only in the Linux Docker container for this server MVP")
}
