package motivation

import (
	"context"
	"errors"
	"time"
)

var errEmptyQuote = errors.New("empty motivation quote")

func ResolveDailyQuote(ctx context.Context, settings Settings, now time.Time, provider Provider) (Settings, *Quote, error) {
	normalized := settings.Normalized()
	if provider == nil {
		provider = NewOllamaProvider(nil)
	}

	quote, err := provider.Generate(ctx, normalized)
	if err != nil {
		normalized.LastError = err.Error()
		return normalized, nil, err
	}
	quote.Text = sanitizeQuote(quote.Text)
	if quote.Text == "" {
		normalized.LastError = "empty motivation quote"
		return normalized, nil, errEmptyQuote
	}

	normalized.CacheDate = CacheDate(now)
	normalized.CachedQuote = quote.Text
	normalized.LastError = ""
	return normalized, &quote, nil
}
