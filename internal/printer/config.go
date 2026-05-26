package printer

import (
	"fmt"
	"net/netip"
	"strings"
)

type Config struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func DefaultConfig() Config {
	return Config{
		Host: "192.168.0.118",
		Port: 5555,
	}
}

func (c Config) Normalized() Config {
	c.Host = strings.TrimSpace(c.Host)
	return c
}

func (c Config) Validate() error {
	normalized := c.Normalized()
	if normalized.Host == "" {
		return fmt.Errorf("IP address is required")
	}
	if _, err := netip.ParseAddr(normalized.Host); err != nil {
		return fmt.Errorf("IP address is invalid")
	}
	if normalized.Port < 1 || normalized.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
