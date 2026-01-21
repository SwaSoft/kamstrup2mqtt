package main

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Device != "/dev/ttyUSB0" {
		t.Errorf("expected default device /dev/ttyUSB0, got %s", cfg.Device)
	}
	if cfg.Baud != 2400 {
		t.Errorf("expected default baud 2400, got %d", cfg.Baud)
	}
}

func TestConfigIO(t *testing.T) {
	// Test LoadConfig with non-existent file
	_, err := LoadConfig("non-existent-file.json")
	if err == nil {
		t.Error("expected error for non-existent config file")
	}

	// Test LoadConfig with valid JSON
	tmpFile, err := os.CreateTemp("", "config*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{"device": "/dev/test", "baud": 9600}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Errorf("failed to load config: %v", err)
	}
	if cfg.Device != "/dev/test" || cfg.Baud != 9600 {
		t.Errorf("loaded config mismatch: %+v", cfg)
	}
}

func TestBuildConfig(t *testing.T) {
	// Create a temp config file
	tmpFile, err := os.CreateTemp("", "config*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{"device": "/dev/file-device", "baud": 4800, "topic": "file/topic"}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Test case: flags should override file
	cfg, err := buildConfig(
		tmpFile.Name(),     // config file
		"/dev/flag-device", // device flag
		9600,               // baud flag
		"",                 // broker (empty, should use default or file)
		"",                 // topic (empty, should use file)
		"user",             // user flag
		"pass",             // pass flag
		"client-123",       // clientID flag
		1,                  // qos flag
	)
	if err != nil {
		t.Errorf("unexpected error in buildConfig: %v", err)
	}

	if cfg.Device != "/dev/flag-device" {
		t.Errorf("expected device flag /dev/flag-device, got %s", cfg.Device)
	}
	if cfg.Baud != 9600 {
		t.Errorf("expected baud flag 9600, got %d", cfg.Baud)
	}
	if cfg.Topic != "file/topic" {
		t.Errorf("expected topic from file file/topic, got %s", cfg.Topic)
	}
	if cfg.User != "user" {
		t.Errorf("expected user flag user, got %s", cfg.User)
	}
	if cfg.Password != "pass" {
		t.Errorf("expected pass flag pass, got %s", cfg.Password)
	}
	if cfg.ClientID != "client-123" {
		t.Errorf("expected clientid flag client-123, got %s", cfg.ClientID)
	}
	if cfg.QoS != 1 {
		t.Errorf("expected qos flag 1, got %d", cfg.QoS)
	}

	// Test case: defaults when no file or flags
	cfg2, err := buildConfig("", "", 0, "", "", "", "", "", -1)
	if err != nil {
		t.Errorf("unexpected error in buildConfig defaults: %v", err)
	}
	defaultCfg := DefaultConfig()
	if cfg2.Device != defaultCfg.Device {
		t.Errorf("expected default device, got %s", cfg2.Device)
	}

	// Test case: Error on missing config file
	_, err = buildConfig("missing-file.json", "", 0, "", "", "", "", "", -1)
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}

	// Test case: Error on invalid QoS
	_, err = buildConfig("", "", 0, "", "", "", "", "", 5)
	if err == nil {
		t.Error("expected error for invalid QoS, got nil")
	}

	// Test case: All fields from file
	tmpFile2, err := os.CreateTemp("", "config_all*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile2.Name())

	contentAll := `{
		"device": "/dev/file",
		"baud": 1200,
		"broker": "tcp://file:1883",
		"topic": "file/topic",
		"user": "file-user",
		"password": "file-password",
		"client_id": "file-client",
		"qos": 2
	}`
	if _, err := tmpFile2.Write([]byte(contentAll)); err != nil {
		t.Fatal(err)
	}
	tmpFile2.Close()

	cfg3, err := buildConfig(tmpFile2.Name(), "", 0, "", "", "", "", "", -1)
	if err != nil {
		t.Errorf("unexpected error in buildConfig with all fields: %v", err)
	}
	if cfg3.Device != "/dev/file" || cfg3.Baud != 1200 || cfg3.Broker != "tcp://file:1883" ||
		cfg3.Topic != "file/topic" || cfg3.User != "file-user" || cfg3.Password != "file-password" ||
		cfg3.ClientID != "file-client" || cfg3.QoS != 2 {
		t.Errorf("config with all file fields mismatch: %+v", cfg3)
	}
}
