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
func DefaultConfig(hostname string, pid int) Config {
	return Config{
		Device:   "/dev/ttyUSB0",
		Baud:     2400,
		Broker:   "tcp://localhost:1883",
		Topic:    "serial/bridge",
		ClientID: fmt.Sprintf("serial2mqtt-%s-%d", hostname, pid),
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
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	pid := os.Getpid()

	base := DefaultConfig(hostname, pid)

	var fileCfg Config
	if configPath != "" {
		var err error
		fileCfg, err = LoadConfig(configPath)
		if err != nil {
			return base, fmt.Errorf("error loading config file: %v", err)
		}
	}

	cliFlags := Config{
		Device:   device,
		Baud:     baud,
		Broker:   broker,
		Topic:    topic,
		User:     user,
		Password: pass,
		ClientID: clientID,
		QoS:      qos,
	}

	logic := NewBridgeLogic(0) // InactivityTimeout not needed for merging
	cfg := logic.MergeConfigs(base, fileCfg, cliFlags)

	if err := logic.ValidateConfig(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
