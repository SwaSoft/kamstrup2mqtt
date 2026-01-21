package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all application settings.
type Config struct {
	Device   string `json:"device"`
	Baud     int    `json:"baud"`
	Broker   string `json:"broker"`
	Topic    string `json:"topic"`
	User     string `json:"user"`
	Password string `json:"password"`
	ClientID string `json:"client_id"`
	QoS      int    `json:"qos"`
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() Config {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return Config{
		Device:   "/dev/ttyUSB0",
		Baud:     2400,
		Broker:   "tcp://localhost:1883",
		Topic:    "serial/bridge",
		ClientID: fmt.Sprintf("serial2mqtt-%s-%d", hostname, os.Getpid()),
		QoS:      0,
	}
}

// LoadConfig loads configuration from a JSON file.
func LoadConfig(path string) (Config, error) {
	var config Config
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}

func buildConfig(configPath, device string, baud int, broker, topic, user, pass, clientID string, qos int) (Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		fileCfg, err := LoadConfig(configPath)
		if err != nil {
			return cfg, fmt.Errorf("error loading config file: %v", err)
		}
		if fileCfg.Device != "" {
			cfg.Device = fileCfg.Device
		}
		if fileCfg.Baud != 0 {
			cfg.Baud = fileCfg.Baud
		}
		if fileCfg.Broker != "" {
			cfg.Broker = fileCfg.Broker
		}
		if fileCfg.Topic != "" {
			cfg.Topic = fileCfg.Topic
		}
		if fileCfg.User != "" {
			cfg.User = fileCfg.User
		}
		if fileCfg.Password != "" {
			cfg.Password = fileCfg.Password
		}
		if fileCfg.ClientID != "" {
			cfg.ClientID = fileCfg.ClientID
		}
		if fileCfg.QoS != 0 {
			cfg.QoS = fileCfg.QoS
		}
	}

	if device != "" {
		cfg.Device = device
	}
	if baud != 0 {
		cfg.Baud = baud
	}
	if broker != "" {
		cfg.Broker = broker
	}
	if topic != "" {
		cfg.Topic = topic
	}
	if user != "" {
		cfg.User = user
	}
	if pass != "" {
		cfg.Password = pass
	}
	if clientID != "" {
		cfg.ClientID = clientID
	}
	if qos >= 0 {
		cfg.QoS = qos
	}

	if cfg.QoS < 0 || cfg.QoS > 2 {
		return cfg, fmt.Errorf("invalid QoS level %d (must be 0, 1, or 2)", cfg.QoS)
	}

	return cfg, nil
}
