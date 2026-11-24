package bridge

import (
	"testing"
	"time"

	"kamstrup2mqtt/internal/config"
)

func TestValidateConfig(t *testing.T) {
	l := NewBridgeLogic(time.Minute)
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{"qos nil", config.Config{}, true},
		{"qos negative", config.Config{QoS: intPtr(-1)}, true},
		{"qos too high", config.Config{QoS: intPtr(3)}, true},
		{"qos 0", config.Config{QoS: intPtr(0)}, false},
		{"qos 1", config.Config{QoS: intPtr(1)}, false},
		{"qos 2", config.Config{QoS: intPtr(2)}, false},
		{"cert without key", config.Config{QoS: intPtr(0), TLSCert: "c"}, true},
		{"key without cert", config.Config{QoS: intPtr(0), TLSKey: "k"}, true},
		{"cert and key", config.Config{QoS: intPtr(0), TLSCert: "c", TLSKey: "k"}, false},
		{"ca only", config.Config{QoS: intPtr(1), TLSCa: "ca"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := l.ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeConfigs_AllFieldsAndOrder(t *testing.T) {
	l := NewBridgeLogic(0)
	base := config.DefaultConfig("h", 1)

	// no overrides returns base
	got := l.MergeConfigs(base)
	if got.Device != base.Device || *got.QoS != 0 {
		t.Errorf("base changed: %+v", got)
	}

	// empty override changes nothing
	got = l.MergeConfigs(base, config.Config{})
	if got.Broker != base.Broker || got.Topic != base.Topic || got.ClientID != base.ClientID {
		t.Errorf("empty override changed base: %+v", got)
	}

	full := config.Config{
		Device: "d", Broker: "b", Topic: "t", User: "u", Password: "p",
		ClientID: "c", QoS: intPtr(2), TLSCa: "ca", TLSCert: "ce", TLSKey: "k",
		TLSInsecure: boolPtr(true),
	}
	got = l.MergeConfigs(base, full)
	if got.Device != "d" || got.Broker != "b" || got.Topic != "t" || got.User != "u" ||
		got.Password != "p" || got.ClientID != "c" || *got.QoS != 2 || got.TLSCa != "ca" ||
		got.TLSCert != "ce" || got.TLSKey != "k" || got.TLSInsecure == nil || !*got.TLSInsecure {
		t.Errorf("full override not applied: %+v", got)
	}

	// later wins; unset later fields do not clear earlier ones
	got = l.MergeConfigs(base, config.Config{Topic: "a", QoS: intPtr(1)}, config.Config{Topic: "b", TLSInsecure: boolPtr(false)})
	if got.Topic != "b" || *got.QoS != 1 || got.TLSInsecure == nil || *got.TLSInsecure {
		t.Errorf("order wrong: %+v", got)
	}

	// base not mutated in value fields
	if base.Topic != "serial/bridge" {
		t.Errorf("base mutated")
	}
}

func TestCalculateTickerInterval_Boundaries(t *testing.T) {
	l := NewBridgeLogic(0)
	def := 10 * time.Second
	if got := l.CalculateTickerInterval(true, 1*time.Nanosecond, def); got != def {
		t.Errorf("half rounds to zero should keep default, got %v", got)
	}
	if got := l.CalculateTickerInterval(true, 2*time.Nanosecond, def); got != 1*time.Nanosecond {
		t.Errorf("got %v", got)
	}
	if got := l.CalculateTickerInterval(true, 2*def, def); got != def {
		t.Errorf("half equal to default keeps default, got %v", got)
	}
	if got := l.CalculateTickerInterval(true, -time.Second, def); got != def {
		t.Errorf("negative watchdog, got %v", got)
	}
	if got := l.CalculateTickerInterval(false, time.Second, def); got != def {
		t.Errorf("disabled, got %v", got)
	}
}

func TestShouldWarnInactivity_Boundaries(t *testing.T) {
	l := NewBridgeLogic(time.Minute)
	now := time.Now()
	// exactly at timeout: no warning
	if w, lw := l.ShouldWarnInactivity(now.Add(-time.Minute), now.Add(-time.Hour), now); w || !lw.Equal(now.Add(-time.Hour)) {
		t.Errorf("at timeout should not warn")
	}
	// just over timeout, warning also old: warn and update
	if w, lw := l.ShouldWarnInactivity(now.Add(-time.Minute-1), now.Add(-time.Hour), now); !w || !lw.Equal(now) {
		t.Errorf("should warn")
	}
	// recently warned: suppressed
	last := now.Add(-time.Second)
	if w, lw := l.ShouldWarnInactivity(now.Add(-time.Hour), last, now); w || !lw.Equal(last) {
		t.Errorf("should be suppressed")
	}
}

func TestCalculateBackoff_Boundaries(t *testing.T) {
	l := NewBridgeLogic(0)
	if got := l.CalculateBackoff(0, time.Minute); got != 0 {
		t.Errorf("zero doubles to zero, got %v", got)
	}
	if got := l.CalculateBackoff(30*time.Second, time.Minute); got != time.Minute {
		t.Errorf("got %v", got)
	}
	if got := l.CalculateBackoff(time.Minute, time.Minute); got != time.Minute {
		t.Errorf("got %v", got)
	}
	if got := l.CalculateBackoff(time.Second, 0); got != 0 {
		t.Errorf("max zero caps to zero, got %v", got)
	}
}

func TestShouldLogSerialOpen_Boundaries(t *testing.T) {
	l := NewBridgeLogic(0)
	if l.ShouldLogSerialOpen(false, time.Second) {
		t.Error("exactly 1s should not log")
	}
	if !l.ShouldLogSerialOpen(false, time.Second+1) {
		t.Error("just over 1s should log")
	}
	if !l.ShouldLogSerialOpen(true, 0) {
		t.Error("first attempt should log")
	}
}
