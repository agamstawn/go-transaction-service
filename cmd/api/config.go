package main

import (
	"errors"
	"strconv"
	"strings"
)

// config stores the application's runtime configuration.
type config struct {
	DatabaseURL   string
	APIKey        string
	APIAccountID  int64
	ListenAddress string
}

// loadConfig reads and validates application configuration.
func loadConfig(
	getenv func(string) string,
) (config, error) {
	if getenv == nil {
		return config{}, errors.New(
			"configuration source is required",
		)
	}

	listenAddress := strings.TrimSpace(getenv("LISTEN_ADDRESS"))

	if listenAddress == "" {
		listenAddress = "127.0.0.1:8080"
	}

	if getenv == nil {
		return config{}, errors.New(
			"configuration source is required",
		)
	}

	databaseURL := getenv("DATABASE_URL")
	apiKey := getenv("API_KEY")

	// Require a database connection string.
	if strings.TrimSpace(databaseURL) == "" {
		return config{}, errors.New(
			"DATABASE_URL is required",
		)
	}

	accountID, err := strconv.ParseInt(
		strings.TrimSpace(getenv("API_ACCOUNT_ID")),
		10,
		64,
	)

	if err != nil || accountID <= 0 {
		return config{}, errors.New(
			"API_ACCOUNT_ID must be a positive integer",
		)
	}

	// Require an API key to protect private endpoints.
	if strings.TrimSpace(apiKey) == "" {
		return config{}, errors.New(
			"API_KEY is required",
		)
	}

	return config{
		DatabaseURL:   databaseURL,
		APIKey:        apiKey,
		APIAccountID:  accountID,
		ListenAddress: listenAddress,
	}, nil

}
