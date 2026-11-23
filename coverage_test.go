package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateConfig(t *testing.T) {
	l := NewBridgeLogic(time.Minute)
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"qos nil", Config{}, true},
		{"qos negative", Config{QoS: intPtr(-1)}, true},
		{"qos too high", Config{QoS: intPtr(3)}, true},
		{"qos 0", Config{QoS: intPtr(0)}, false},
		{"qos 1", Config{QoS: intPtr(1)}, false},
		{"qos 2", Config{QoS: intPtr(2)}, false},
		{"cert without key", Config{QoS: intPtr(0), TLSCert: "c"}, true},
		{"key without cert", Config{QoS: intPtr(0), TLSKey: "k"}, true},
		{"cert and key", Config{QoS: intPtr(0), TLSCert: "c", TLSKey: "k"}, false},
		{"ca only", Config{QoS: intPtr(1), TLSCa: "ca"}, false},
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
	base := DefaultConfig("h", 1)

	// no overrides returns base
	got := l.MergeConfigs(base)
	if got.Device != base.Device || *got.QoS != 0 {
		t.Errorf("base changed: %+v", got)
	}

	// empty override changes nothing
	got = l.MergeConfigs(base, Config{})
	if got.Broker != base.Broker || got.Topic != base.Topic || got.ClientID != base.ClientID {
		t.Errorf("empty override changed base: %+v", got)
	}

	full := Config{
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
	got = l.MergeConfigs(base, Config{Topic: "a", QoS: intPtr(1)}, Config{Topic: "b", TLSInsecure: boolPtr(false)})
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

func TestLoadCredentials_EdgeCases(t *testing.T) {
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
	u, p, err := LoadCredentials(write("  user  \r\n  pass  \r\n"))
	if err != nil || u != "user" || p != "pass" {
		t.Errorf("got %q %q %v", u, p, err)
	}
	u, p, err = LoadCredentials(write("user\npass\nextra\n"))
	if err != nil || u != "user" || p != "pass" {
		t.Errorf("extra lines: %q %q %v", u, p, err)
	}
	u, p, err = LoadCredentials(write("user\n\npass"))
	if err != nil || u != "user" || p != "" {
		t.Errorf("blank password line: %q %q %v", u, p, err)
	}
}

func TestDefaultConfig_Values(t *testing.T) {
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

func feed(p *FrameParser, data []byte) (frames [][]byte, errs int) {
	for _, b := range data {
		f, err := p.ProcessByte(b)
		if err != nil {
			errs++
		}
		if f != nil {
			frames = append(frames, f)
		}
	}
	return
}

// mkFrame builds a frame of total size n (delimiters at both ends).
func mkFrame(n int) []byte {
	length := n - 2
	f := make([]byte, n)
	f[0] = Delimiter
	f[1] = byte(length >> 8 & 0x0F)
	f[2] = byte(length & 0xFF)
	for i := 3; i < n-1; i++ {
		f[i] = 0x01
	}
	f[n-1] = Delimiter
	return f
}

func TestFrameParser_SizeBoundaries(t *testing.T) {
	for _, n := range []int{MinPacketSize, MinPacketSize + 1, MaxPacketSize - 1, MaxPacketSize} {
		p := NewFrameParser()
		frames, errs := feed(p, mkFrame(n))
		if len(frames) != 1 || errs != 0 || len(frames[0]) != n {
			t.Errorf("size %d: frames=%d errs=%d", n, len(frames), errs)
		}
	}
	// just below min and just above max are rejected
	for _, length := range []int{MinPacketSize - 3, MaxPacketSize - 1} {
		p := NewFrameParser()
		_, errs := feed(p, []byte{Delimiter, byte(length >> 8 & 0x0F), byte(length)})
		if errs != 1 {
			t.Errorf("length %d: errs=%d", length, errs)
		}
	}
}

func TestFrameParser_IgnoresLeadingGarbageAndBackToBack(t *testing.T) {
	p := NewFrameParser()
	data := append([]byte{0x00, 0x01, 0xFF}, mkFrame(12)...)
	data = append(data, mkFrame(MinPacketSize)...)
	frames, errs := feed(p, data)
	if len(frames) != 2 || errs != 0 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_SplitDeliveryAndResetMidFrame(t *testing.T) {
	p := NewFrameParser()
	f := mkFrame(20)
	if frames, _ := feed(p, f[:7]); len(frames) != 0 {
		t.Fatal("no frame expected from partial data")
	}
	p.Reset()
	if p.frameStarted || p.byteCounter != 0 || p.expectedPacketSize != 0 || len(p.frameBuffer) != 0 {
		t.Errorf("state not cleared: %+v", p)
	}
	// remainder alone (no leading delimiter) must not yield a frame
	if frames, _ := feed(p, f[7:len(f)-1]); len(frames) != 0 {
		t.Error("unexpected frame")
	}
	// Reset on a fresh parser is harmless, and a full frame then parses
	p.Reset()
	if frames, errs := feed(p, f); len(frames) != 1 || errs != 0 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_ReturnedFrameIsCopy(t *testing.T) {
	p := NewFrameParser()
	frames, _ := feed(p, mkFrame(12))
	if len(frames) != 1 {
		t.Fatal("expected frame")
	}
	first := append([]byte(nil), frames[0]...)
	feed(p, mkFrame(12)[:5]) // reuses the internal buffer
	for i := range first {
		if frames[0][i] != first[i] {
			t.Fatal("returned frame aliased internal buffer")
		}
	}
}

func TestFrameParser_ErrorRecovery(t *testing.T) {
	p := NewFrameParser()
	bad := mkFrame(12)
	bad[11] = 0x00 // wrong terminator
	frames, errs := feed(p, bad)
	if len(frames) != 0 || errs != 1 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
	if frames, errs := feed(p, mkFrame(12)); len(frames) != 1 || errs != 0 {
		t.Errorf("no recovery: frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_ResyncOnEarlyDelimiter(t *testing.T) {
	p := NewFrameParser()
	data := append([]byte{Delimiter}, mkFrame(14)...)
	frames, errs := feed(p, data)
	if len(frames) != 1 || errs != 0 || len(frames[0]) != 14 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
}
