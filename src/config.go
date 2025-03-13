package src

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Scraper ScraperConfig `yaml:"scraper"`
	Checker CheckerConfig `yaml:"checker"`
}

// ScraperConfig defines settings for proxy scraping
type ScraperConfig struct {
	Timeout    time.Duration `yaml:"timeout"`
	UserAgent  string        `yaml:"user_agent"`
	Concurrent int          `yaml:"concurrent"`
	UserAgents []string     `yaml:"user_agents"`
}

// CheckerConfig defines settings for proxy checking
type CheckerConfig struct {
	Timeout          time.Duration `yaml:"timeout"`
	ConnectTimeout   time.Duration `yaml:"connect_timeout"`
	Concurrent       int           `yaml:"concurrent"`
	ConcurrentHTTP   int           `yaml:"concurrent_http"`
	ConcurrentSOCKS5 int           `yaml:"concurrent_socks5"`
	CheckURLs        []string      `yaml:"check_urls"`
	TestURL          string        `yaml:"test_url"`
	UserAgent        string        `yaml:"user_agent"`
	StrictCheck      bool          `yaml:"strict_check"`      // Enable strict checking mode
	DetailedOutput   bool          `yaml:"detailed_output"`   // Enable detailed output (only works with strict_check)
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Set default values if not specified
	if config.Scraper.Timeout == 0 {
		config.Scraper.Timeout = 10 * time.Second
	}
	if config.Scraper.UserAgent == "" {
		config.Scraper.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	}
	if len(config.Scraper.UserAgents) == 0 {
		config.Scraper.UserAgents = []string{config.Scraper.UserAgent}
	}
	if config.Scraper.Concurrent == 0 {
		config.Scraper.Concurrent = 10
	}

	// Checker defaults
	if config.Checker.Timeout == 0 {
		if config.Checker.StrictCheck {
			config.Checker.Timeout = 3 * time.Second
		} else {
			config.Checker.Timeout = 10 * time.Second
		}
	}
	if config.Checker.ConnectTimeout == 0 {
		if config.Checker.StrictCheck {
			config.Checker.ConnectTimeout = 3 * time.Second
		} else {
			config.Checker.ConnectTimeout = 5 * time.Second
		}
	}
	if config.Checker.Concurrent == 0 {
		config.Checker.Concurrent = 100
	}
	if config.Checker.ConcurrentHTTP == 0 {
		config.Checker.ConcurrentHTTP = config.Checker.Concurrent
	}
	if config.Checker.ConcurrentSOCKS5 == 0 {
		config.Checker.ConcurrentSOCKS5 = config.Checker.Concurrent
	}
	if len(config.Checker.CheckURLs) == 0 {
		config.Checker.CheckURLs = []string{"http://checkip.amazonaws.com"}
	}
	if config.Checker.TestURL == "" {
		config.Checker.TestURL = config.Checker.CheckURLs[0]
	}
	if config.Checker.UserAgent == "" {
		config.Checker.UserAgent = config.Scraper.UserAgent
	}

	// DetailedOutput works only with StrictCheck
	if !config.Checker.StrictCheck {
		config.Checker.DetailedOutput = false
	}

	return &config, nil
} 