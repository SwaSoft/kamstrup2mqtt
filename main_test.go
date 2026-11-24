package main

import (
	"os"
	"testing"

	"kamstrup2mqtt/internal/config"
)

func TestBuildConfig(t *testing.T) {
	// Create a temp config file
	tmpFile, err := os.CreateTemp("", "config*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{"device": "/dev/file-device", "topic": "file/topic"}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Test case: flags should override file
	cfg, err := buildConfig(
		tmpFile.Name(),     // config file
		"",                 // credfile (empty)
		"/dev/flag-device", // device flag
		"",                 // broker (empty, should use default or file)
		"",                 // topic (empty, should use file)
		"user",             // user flag
		"pass",             // pass flag
		"client-123",       // clientID flag
		1,                  // qos flag
		"", "", "", -1,     // TLS options (not set)
	)
	if err != nil {
		t.Errorf("unexpected error in buildConfig: %v", err)
	}

	if cfg.Device != "/dev/flag-device" {
		t.Errorf("expected device flag /dev/flag-device, got %s", cfg.Device)
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
	if cfg.QoS == nil || *cfg.QoS != 1 {
		t.Errorf("expected qos flag 1, got %v", cfg.QoS)
	}

	// Test case: defaults when no file or flags
	cfg2, err := buildConfig("", "", "", "", "", "", "", "", -1, "", "", "", -1)
	if err != nil {
		t.Errorf("unexpected error in buildConfig defaults: %v", err)
	}
	// We need to re-fetch hostname/pid for cfg2 because buildConfig uses real os.Hostname()
	realHost, _ := os.Hostname()
	if realHost == "" {
		realHost = "unknown"
	}
	realPid := os.Getpid()
	realDefault := config.DefaultConfig(realHost, realPid)

	if cfg2.Device != realDefault.Device {
		t.Errorf("expected default device, got %s", cfg2.Device)
	}

	// Test case: Error on missing config file
	_, err = buildConfig("missing-file.json", "", "", "", "", "", "", "", -1, "", "", "", -1)
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}

	// Test case: Error on invalid QoS
	_, err = buildConfig("", "", "", "", "", "", "", "", 5, "", "", "", -1)
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

	cfg3, err := buildConfig(tmpFile2.Name(), "", "", "", "", "", "", "", -1, "", "", "", -1)
	if err != nil {
		t.Errorf("unexpected error in buildConfig with all fields: %v", err)
	}
	if cfg3.Device != "/dev/file" || cfg3.Broker != "tcp://file:1883" ||
		cfg3.Topic != "file/topic" || cfg3.User != "file-user" || cfg3.Password != "file-password" ||
		cfg3.ClientID != "file-client" || cfg3.QoS == nil || *cfg3.QoS != 2 {
		t.Errorf("config with all file fields mismatch: %+v", cfg3)
	}
}

func TestCredfilePriority(t *testing.T) {
	// Create a config file with user/pass
	configFile, err := os.CreateTemp("", "config*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(configFile.Name())

	configContent := `{"user": "config-user", "password": "config-pass"}`
	if _, err := configFile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	configFile.Close()

	// Create a credentials file
	credFile, err := os.CreateTemp("", "creds*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(credFile.Name())

	if _, err := credFile.Write([]byte("cred-user\ncred-pass")); err != nil {
		t.Fatal(err)
	}
	credFile.Close()

	// Test: credfile should override config.json
	cfg, err := buildConfig(configFile.Name(), credFile.Name(), "", "", "", "", "", "", -1, "", "", "", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.User != "cred-user" {
		t.Errorf("expected credfile user 'cred-user', got '%s'", cfg.User)
	}
	if cfg.Password != "cred-pass" {
		t.Errorf("expected credfile password 'cred-pass', got '%s'", cfg.Password)
	}

	// Test: CLI flags should override credfile
	cfg2, err := buildConfig(configFile.Name(), credFile.Name(), "", "", "", "cli-user", "cli-pass", "", -1, "", "", "", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.User != "cli-user" {
		t.Errorf("expected CLI user 'cli-user', got '%s'", cfg2.User)
	}
	if cfg2.Password != "cli-pass" {
		t.Errorf("expected CLI password 'cli-pass', got '%s'", cfg2.Password)
	}

	// Test: error on missing credfile
	_, err = buildConfig("", "missing-creds.txt", "", "", "", "", "", "", -1, "", "", "", -1)
	if err == nil {
		t.Error("expected error for missing credentials file")
	}
}

func TestTLSConfig(t *testing.T) {
	// Test: TLS cert without key should fail validation
	_, err := buildConfig("", "", "", "", "", "", "", "", -1, "", "/path/to/cert.pem", "", -1)
	if err == nil {
		t.Error("expected error when tls_cert specified without tls_key")
	}

	// Test: TLS key without cert should fail validation
	_, err = buildConfig("", "", "", "", "", "", "", "", -1, "", "", "/path/to/key.pem", -1)
	if err == nil {
		t.Error("expected error when tls_key specified without tls_cert")
	}

	// Test: TLS config from JSON file
	tmpFile, err := os.CreateTemp("", "config_tls*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{
		"broker": "ssl://broker:8883",
		"tls_ca": "/etc/certs/ca.pem",
		"tls_cert": "/etc/certs/client.pem",
		"tls_key": "/etc/certs/client.key",
		"tls_insecure": false
	}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := buildConfig(tmpFile.Name(), "", "", "", "", "", "", "", -1, "", "", "", -1)
	if err != nil {
		t.Fatalf("unexpected error loading TLS config: %v", err)
	}
	if cfg.TLSCa != "/etc/certs/ca.pem" {
		t.Errorf("expected tls_ca '/etc/certs/ca.pem', got '%s'", cfg.TLSCa)
	}
	if cfg.TLSCert != "/etc/certs/client.pem" {
		t.Errorf("expected tls_cert '/etc/certs/client.pem', got '%s'", cfg.TLSCert)
	}
	if cfg.TLSKey != "/etc/certs/client.key" {
		t.Errorf("expected tls_key '/etc/certs/client.key', got '%s'", cfg.TLSKey)
	}
	if cfg.TLSInsecure == nil || *cfg.TLSInsecure != false {
		t.Errorf("expected tls_insecure false, got %v", cfg.TLSInsecure)
	}

	// Test: CLI TLS flags override file
	cfg2, err := buildConfig(tmpFile.Name(), "", "", "", "", "", "", "", -1, "/cli/ca.pem", "", "", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.TLSCa != "/cli/ca.pem" {
		t.Errorf("expected CLI tls_ca '/cli/ca.pem', got '%s'", cfg2.TLSCa)
	}
	if cfg2.TLSInsecure == nil || *cfg2.TLSInsecure != true {
		t.Errorf("expected CLI tls_insecure true, got %v", cfg2.TLSInsecure)
	}
}
