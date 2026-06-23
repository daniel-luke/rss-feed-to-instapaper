package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Feed struct {
	URL   string `yaml:"url"`
	Label string `yaml:"label"`
}

type Config struct {
	MaxAgeDays           int    `yaml:"max_age_days"`
	ArchiveRetentionDays int    `yaml:"archive_retention_days"`
	ClearArchiveOnSync   bool   `yaml:"clear_archive_on_sync"`
	SortByDate           *bool  `yaml:"sort_by_date"`
	Feeds                []Feed `yaml:"feeds"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Feeds) == 0 {
		return nil, fmt.Errorf("config has no feeds")
	}
	for i, f := range cfg.Feeds {
		if f.URL == "" {
			return nil, fmt.Errorf("feed %d missing url", i)
		}
	}
	if cfg.MaxAgeDays == 0 {
		cfg.MaxAgeDays = 1
	}
	if cfg.SortByDate == nil {
		t := true
		cfg.SortByDate = &t
	}
	return &cfg, nil
}
