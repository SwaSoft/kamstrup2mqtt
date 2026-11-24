package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig("host", 42)
	if c.Device != "/dev/ttyUSB0" || c.Broker != "tcp://localhost:1883" || c.Topic != "serial/bridge" {
		t.Errorf("defaults: %+v", c)
	}
	if c.ClientID != "kamstrup2mqtt-host-42" {
		t.Errorf("client id %q", c.ClientID)
	}
	if c.QoS == nil || *c.QoS != 0 || c.TLSInsecure != nil {
		t.Errorf("qos/tls: %+v", c)
	}
}

func TestConfigIO(t *testing.T) {
	if _, err := LoadConfig("non-existent-file.json"); err == nil {
		t.Error("expected error for non-existent config file")
	}

	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"device": "/dev/test", "broker": "tcp://test:1883"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Device != "/dev/test" || cfg.Broker != "tcp://test:1883" {
		t.Errorf("loaded config mismatch: %+v", cfg)
	}
}

func TestLoadConfig_Malformed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if _, err := LoadConfig(write("bad.json", "{not json")); err == nil {
		t.Error("expected error for malformed JSON")
	}
	if _, err := LoadConfig(write("empty.json", "")); err == nil {
		t.Error("expected error for empty file")
	}
	if _, err := LoadConfig(write("type.json", `{"qos":"high"}`)); err == nil {
		t.Error("expected error for wrong type")
	}
	cfg, err := LoadConfig(write("obj.json", `{}`))
	if err != nil || cfg.QoS != nil || cfg.TLSInsecure != nil {
		t.Errorf("empty object: %+v %v", cfg, err)
	}
	cfg, err = LoadConfig(write("ok.json", `{"qos":2,"tls_insecure":false,"tls_ca":"x","unknown":1}`))
	if err != nil || cfg.QoS == nil || *cfg.QoS != 2 || cfg.TLSInsecure == nil || *cfg.TLSInsecure || cfg.TLSCa != "x" {
		t.Errorf("parsed: %+v %v", cfg, err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Error("expected error reading a directory")
	}
}

func TestLoadCredentials(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		p := filepath.Join(dir, "cred")
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if _, _, err := LoadCredentials(filepath.Join(dir, "missing")); err == nil {
		t.Error("expected error for missing file")
	}
	if _, _, err := LoadCredentials(write("")); err == nil {
		t.Error("expected error for empty file")
	}
	if _, _, err := LoadCredentials(write("onlyuser\n")); err == nil {
		t.Error("expected error for single line")
	}

	tests := []struct {
		name, content, user, pass string
	}{
		{"plain", "testuser\ntestpass", "testuser", "testpass"},
		{"whitespace", "  spaceduser  \n  spacedpass  \n", "spaceduser", "spacedpass"},
		{"crlf", "  user  \r\n  pass  \r\n", "user", "pass"},
		{"special chars", "user@domain.com\npass:with:colons!@#$", "user@domain.com", "pass:with:colons!@#$"},
		{"extra lines", "user\npass\nextra\n", "user", "pass"},
		{"blank password line", "user\n\npass", "user", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, p, err := LoadCredentials(write(tt.content))
			if err != nil || u != tt.user || p != tt.pass {
				t.Errorf("got %q %q %v", u, p, err)
			}
		})
	}
}
