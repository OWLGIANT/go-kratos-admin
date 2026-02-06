package config

import (
	"fmt"
	"os"

	"actor/helper"
	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Exchanges []ExchangeConfig `yaml:"exchanges"`
	Log       LogConfig        `yaml:"log"`
	IPPool    IPPoolConfig     `yaml:"ippool"`
}

// ServerConfig represents the server settings
type ServerConfig struct {
	HTTPPort int    `yaml:"http_port"`
	GRPCPort int    `yaml:"grpc_port"`
	WSPath   string `yaml:"ws_path"`
	Nickname string `yaml:"nickname"` // 托管者昵称 (必须)
}

// ExchangeConfig represents an exchange configuration
type ExchangeConfig struct {
	Name       string   `yaml:"name"`
	APIKey     string   `yaml:"api_key"`
	SecretKey  string   `yaml:"secret_key"`
	PassKey    string   `yaml:"passphrase"`
	RobotID    string   `yaml:"robot_id"`
	ProxyURL   string   `yaml:"proxy_url"`
	LocalAddr  string   `yaml:"local_addr"`
	Symbols    []string `yaml:"symbols"`    // Trading symbols to subscribe
	SymbolAll  bool     `yaml:"symbol_all"` // Subscribe to all symbols
	NeedAuth   bool     `yaml:"need_auth"`  // Enable private data subscription
	DepthLevel int      `yaml:"depth_level"` // Depth level for orderbook
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxBackups int    `yaml:"max_backups"`
}

// IPPoolConfig represents IP pool configuration
type IPPoolConfig struct {
	AutoGenerate bool `yaml:"auto_generate"`
	Interval     int  `yaml:"interval"` // seconds
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.HTTPPort == 0 {
		cfg.Server.HTTPPort = 8080
	}
	if cfg.Server.GRPCPort == 0 {
		cfg.Server.GRPCPort = 9090
	}
	if cfg.Server.WSPath == "" {
		cfg.Server.WSPath = "/ws"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.MaxBackups == 0 {
		cfg.Log.MaxBackups = 7
	}

	return &cfg, nil
}

// ToBrokerConfig converts ExchangeConfig to helper.BrokerConfig
func (ec *ExchangeConfig) ToBrokerConfig() helper.BrokerConfig {
	// Parse symbols into Pairs
	pairs := make([]helper.Pair, 0, len(ec.Symbols))
	for _, symbol := range ec.Symbols {
		pair, err := helper.StringPairToPair(symbol)
		if err != nil {
			fmt.Printf("Warning: failed to parse symbol %s: %v\n", symbol, err)
			continue
		}
		pairs = append(pairs, pair)
	}

	// If no symbols specified but SymbolAll is true, add a default pair
	if len(pairs) == 0 && ec.SymbolAll {
		pair, _ := helper.StringPairToPair("BTC_USDT")
		pairs = append(pairs, pair)
	}

	return helper.BrokerConfig{
		Name:                   ec.Name,
		AccessKey:              ec.APIKey,
		SecretKey:              ec.SecretKey,
		PassKey:                ec.PassKey,
		ProxyURL:               ec.ProxyURL,
		LocalAddr:              ec.LocalAddr,
		NeedTicker:             true,
		NeedAuth:               ec.NeedAuth,
		WsDepthLevel:           ec.DepthLevel,
		RobotId:                ec.RobotID,
		Pairs:                  pairs,
		SymbolAll:              ec.SymbolAll,
		PushTickerEvenEmpty:    false,
		NeedTrade:              true,
		NeedIndex:              true,
		DisableAutoUpdateDepth: false,
	}
}
