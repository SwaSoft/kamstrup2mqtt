package main

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("test-host", 1234)
	if cfg.Device != "/dev/ttyUSB0" {
		t.Errorf("expected default device /dev/ttyUSB0, got %s", cfg.Device)
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

	content := `{"device": "/dev/test", "broker": "tcp://test:1883"}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Errorf("failed to load config: %v", err)
	}
	if cfg.Device != "/dev/test" || cfg.Broker != "tcp://test:1883" {
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
	if cfg.QoS != 1 {
		t.Errorf("expected qos flag 1, got %d", cfg.QoS)
	}

	// Test case: defaults when no file or flags
	cfg2, err := buildConfig("", "", "", "", "", "", "", "", -1)
	if err != nil {
		t.Errorf("unexpected error in buildConfig defaults: %v", err)
	}
	// We need to re-fetch hostname/pid for cfg2 because buildConfig uses real os.Hostname()
	realHost, _ := os.Hostname()
	if realHost == "" {
		realHost = "unknown"
	}
	realPid := os.Getpid()
	realDefault := DefaultConfig(realHost, realPid)

	if cfg2.Device != realDefault.Device {
		t.Errorf("expected default device, got %s", cfg2.Device)
	}

	// Test case: Error on missing config file
	_, err = buildConfig("missing-file.json", "", "", "", "", "", "", "", -1)
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}

	// Test case: Error on invalid QoS
	_, err = buildConfig("", "", "", "", "", "", "", "", 5)
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

	cfg3, err := buildConfig(tmpFile2.Name(), "", "", "", "", "", "", "", -1)
	if err != nil {
		t.Errorf("unexpected error in buildConfig with all fields: %v", err)
	}
	if cfg3.Device != "/dev/file" || cfg3.Broker != "tcp://file:1883" ||
		cfg3.Topic != "file/topic" || cfg3.User != "file-user" || cfg3.Password != "file-password" ||
		cfg3.ClientID != "file-client" || cfg3.QoS != 2 {
		t.Errorf("config with all file fields mismatch: %+v", cfg3)
	}
}

func TestLoadCredentials(t *testing.T) {
	// Test valid credentials file
	tmpFile, err := os.CreateTemp("", "creds*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte("testuser\ntestpass")); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	user, pass, err := LoadCredentials(tmpFile.Name())
	if err != nil {
		t.Errorf("unexpected error loading credentials: %v", err)
	}
	if user != "testuser" {
		t.Errorf("expected user 'testuser', got '%s'", user)
	}
	if pass != "testpass" {
		t.Errorf("expected password 'testpass', got '%s'", pass)
	}

	// Test missing file
	_, _, err = LoadCredentials("non-existent-creds.txt")
	if err == nil {
		t.Error("expected error for non-existent credentials file")
	}

	// Test invalid format (single line)
	tmpFile2, err := os.CreateTemp("", "creds_invalid*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile2.Name())

	if _, err := tmpFile2.Write([]byte("onlyuser")); err != nil {
		t.Fatal(err)
	}
	tmpFile2.Close()

	_, _, err = LoadCredentials(tmpFile2.Name())
	if err == nil {
		t.Error("expected error for single-line credentials file")
	}

	// Test whitespace trimming
	tmpFile3, err := os.CreateTemp("", "creds_whitespace*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile3.Name())

	if _, err := tmpFile3.Write([]byte("  spaceduser  \n  spacedpass  \n")); err != nil {
		t.Fatal(err)
	}
	tmpFile3.Close()

	user, pass, err = LoadCredentials(tmpFile3.Name())
	if err != nil {
		t.Errorf("unexpected error loading credentials with whitespace: %v", err)
	}
	if user != "spaceduser" {
		t.Errorf("expected trimmed user 'spaceduser', got '%s'", user)
	}
	if pass != "spacedpass" {
		t.Errorf("expected trimmed password 'spacedpass', got '%s'", pass)
	}

	// Test password with special characters (including colons)
	tmpFile4, err := os.CreateTemp("", "creds_special*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile4.Name())

	if _, err := tmpFile4.Write([]byte("user@domain.com\npass:with:colons!@#$")); err != nil {
		t.Fatal(err)
	}
	tmpFile4.Close()

	user, pass, err = LoadCredentials(tmpFile4.Name())
	if err != nil {
		t.Errorf("unexpected error loading credentials with special chars: %v", err)
	}
	if user != "user@domain.com" {
		t.Errorf("expected user 'user@domain.com', got '%s'", user)
	}
	if pass != "pass:with:colons!@#$" {
		t.Errorf("expected password 'pass:with:colons!@#$', got '%s'", pass)
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
	cfg, err := buildConfig(configFile.Name(), credFile.Name(), "", "", "", "", "", "", -1)
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
	cfg2, err := buildConfig(configFile.Name(), credFile.Name(), "", "", "", "cli-user", "cli-pass", "", -1)
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
	_, err = buildConfig("", "missing-creds.txt", "", "", "", "", "", "", -1)
	if err == nil {
		t.Error("expected error for missing credentials file")
	}
}
