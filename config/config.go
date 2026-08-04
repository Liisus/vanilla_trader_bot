package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"
)

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(path, "config.json"))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var raw rawURL
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	awaitTimeout, err := time.ParseDuration(raw.AwaitTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse awaitTimeout: %w", err)
	}

	cfg := Config{
		URL:          raw.URL,
		AwaitTimeout: awaitTimeout,
		Securities:   make(map[Security]SecurityData),
	}

	secFiles, err := filepath.Glob(filepath.Join(path, "securities", "*.json"))
	if err != nil {
		return Config{}, fmt.Errorf("find securities: %w", err)
	}

	for _, secFile := range secFiles {
		secs, secData, err := parseSecurity(secFile)
		if err != nil {
			return Config{}, err
		}
		for i := range secs {
			cfg.Securities[secs[i]] = secData[i]
		}
	}

	queues, err := parseQueues(filepath.Join(path, "queues.json"))
	if err != nil {
		return Config{}, err
	}
	cfg.Publisher = queues.Publisher
	cfg.Consumers = queues.Consumers

	return cfg, nil
}

func parseQueues(path string) (rawQueues, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rawQueues{}, fmt.Errorf("read queues: %w", err)
	}
	var q rawQueues
	if err := json.Unmarshal(data, &q); err != nil {
		return rawQueues{}, fmt.Errorf("parse queues: %w", err)
	}
	return q, nil
}

func parseSecurity(secFile string) ([]Security, []SecurityData, error) {
	data, err := os.ReadFile(secFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read security %s: %w", secFile, err)
	}
	var rawSecs []rawSecurityData
	if err := json.Unmarshal(data, &rawSecs); err != nil {
		return nil, nil, fmt.Errorf("parse security %s: %w", secFile, err)
	}

	secs := make([]Security, 0)
	secData := make([]SecurityData, 0)

	for _, s := range rawSecs {
		if s.LotSize.Compare(decimal.NewFromInt(0)) <= 0 {
			return nil, nil, errors.New("lot size is not positive")
		}
		if s.MinStep.Compare(decimal.NewFromInt(0)) <= 0 {
			return nil, nil, errors.New("minimal step is not positive")
		}
		secs = append(secs, Security{Market: s.BoardID, Ticker: s.SecID})
		secData = append(secData, SecurityData{LotSize: s.LotSize, MinStep: s.MinStep})
	}

	return secs, secData, nil
}
