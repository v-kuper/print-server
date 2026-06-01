package telegramfax

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL     = "https://api.telegram.org"
	defaultPollTimeout    = 25 * time.Second
	envBotToken           = "TELEGRAM_FAX_BOT_TOKEN"
	envOwnerIDs           = "TELEGRAM_FAX_OWNER_IDS"
	envAllowedSenderIDs   = "TELEGRAM_FAX_ALLOWED_SENDER_IDS"
	envAPIBaseURL         = "TELEGRAM_FAX_API_BASE_URL"
	envPollTimeoutSeconds = "TELEGRAM_FAX_POLL_TIMEOUT_SECONDS"
)

type Config struct {
	Token            string
	APIBaseURL       string
	PollTimeout      time.Duration
	OwnerIDs         IDSet
	AllowedSenderIDs IDSet
}

type IDSet map[int64]struct{}

func NewIDSet(ids ...int64) IDSet {
	set := make(IDSet, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func (s IDSet) Contains(id int64) bool {
	_, ok := s[id]
	return ok
}

func ConfigFromEnv(getenv func(string) string) (Config, bool, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	config := Config{
		Token:       strings.TrimSpace(getenv(envBotToken)),
		APIBaseURL:  strings.TrimRight(strings.TrimSpace(getenv(envAPIBaseURL)), "/"),
		PollTimeout: defaultPollTimeout,
	}
	if config.APIBaseURL == "" {
		config.APIBaseURL = defaultAPIBaseURL
	}
	if config.Token == "" {
		return config, false, nil
	}

	if value := strings.TrimSpace(getenv(envPollTimeoutSeconds)); value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds <= 0 {
			return config, true, fmt.Errorf("%s must be a positive integer", envPollTimeoutSeconds)
		}
		config.PollTimeout = time.Duration(seconds) * time.Second
	}

	var err error
	config.OwnerIDs, err = parseIDSet(envOwnerIDs, getenv(envOwnerIDs))
	if err != nil {
		return config, true, err
	}
	if len(config.OwnerIDs) == 0 {
		return config, true, fmt.Errorf("%s is required when %s is set", envOwnerIDs, envBotToken)
	}
	config.AllowedSenderIDs, err = parseIDSet(envAllowedSenderIDs, getenv(envAllowedSenderIDs))
	if err != nil {
		return config, true, err
	}

	return config, true, nil
}

func parseIDSet(key string, value string) (IDSet, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r'
	})
	set := make(IDSet, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		id, err := strconv.ParseInt(field, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%s contains invalid Telegram user ID %q", key, field)
		}
		set[id] = struct{}{}
	}
	return set, nil
}
