package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// LoadCredentials reads MQTT username and password from a file.
// The file format is two lines: username on first line, password on second.
// This supports systemd's LoadCredential directive for secure credential management.
func LoadCredentials(path string) (user, password string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("credentials file must contain username and password on separate lines")
	}

	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), nil
}

func buildConfig(configPath, credfilePath, device string, baud int, broker, topic, user, pass, clientID string, qos int) (Config, error) {
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

	var credCfg Config
	if credfilePath != "" {
		credUser, credPass, err := LoadCredentials(credfilePath)
		if err != nil {
			return base, fmt.Errorf("error loading credentials: %v", err)
		}
		credCfg.User = credUser
		credCfg.Password = credPass
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
	cfg := logic.MergeConfigs(base, fileCfg, credCfg, cliFlags)

	if err := logic.ValidateConfig(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
