// Package config holds the application configuration and its loading from files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config holds all application settings.
type Config struct {
	Device      string `json:"device"`
	Broker      string `json:"broker"`
	Topic       string `json:"topic"`
	User        string `json:"user"`
	Password    string `json:"password"`
	ClientID    string `json:"client_id"`
	QoS         *int   `json:"qos,omitempty"`
	TLSCa       string `json:"tls_ca,omitempty"`
	TLSCert     string `json:"tls_cert,omitempty"`
	TLSKey      string `json:"tls_key,omitempty"`
	TLSInsecure *bool  `json:"tls_insecure,omitempty"`
}

// intPtr returns a pointer to the given int value.
func intPtr(i int) *int {
	return &i
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig(hostname string, pid int) Config {
	return Config{
		Device:   "/dev/ttyUSB0",
		Broker:   "tcp://localhost:1883",
		Topic:    "serial/bridge",
		ClientID: fmt.Sprintf("kamstrup2mqtt-%s-%d", hostname, pid),
		QoS:      intPtr(0),
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
