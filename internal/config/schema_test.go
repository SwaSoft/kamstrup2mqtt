package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestConfigJSONSchemaUnchanged pins the Config JSON schema the application had
// before the refactor: every key, its Go field and its type are still read from
// a config file, and Config marshals back to the same keys.
func TestConfigJSONSchemaUnchanged(t *testing.T) {
	const doc = `{
		"device": "/dev/ttyAMA0",
		"broker": "ssl://broker:8883",
		"topic": "meter/raw",
		"user": "u",
		"password": "p",
		"client_id": "cid",
		"qos": 1,
		"tls_ca": "/ca.pem",
		"tls_cert": "/cert.pem",
		"tls_key": "/key.pem",
		"tls_insecure": true
	}`
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	qos, insecure := 1, true
	want := Config{
		Device: "/dev/ttyAMA0", Broker: "ssl://broker:8883", Topic: "meter/raw",
		User: "u", Password: "p", ClientID: "cid", QoS: &qos,
		TLSCa: "/ca.pem", TLSCert: "/cert.pem", TLSKey: "/key.pem", TLSInsecure: &insecure,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("loaded %+v, want %+v", cfg, want)
	}

	// The set of JSON keys, with their omitempty behaviour, is unchanged.
	data, err := json.Marshal(Config{})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	wantEmpty := map[string]any{"device": "", "broker": "", "topic": "", "user": "", "password": "", "client_id": ""}
	if !reflect.DeepEqual(keys, wantEmpty) {
		t.Errorf("empty Config marshals to %v, want %v", keys, wantEmpty)
	}
	data, err = json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	keys = nil
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 11 {
		t.Errorf("full Config marshals to %d keys, want 11: %v", len(keys), keys)
	}
}
