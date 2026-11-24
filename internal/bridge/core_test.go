package bridge

import (
	"context"
	"io"
	"math"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"kamstrup2mqtt/internal/config"
	"kamstrup2mqtt/internal/parser"
)

func TestCalculateTickerInterval(t *testing.T) {
	logic := NewBridgeLogic(60 * time.Second)
	defaultInterval := 1 * time.Second

	tests := []struct {
		name             string
		watchdogEnabled  bool
		watchdogInterval time.Duration
		expected         time.Duration
	}{
		{
			name:             "Watchdog disabled",
			watchdogEnabled:  false,
			watchdogInterval: 0,
			expected:         defaultInterval,
		},
		{
			name:             "Watchdog enabled, interval large",
			watchdogEnabled:  true,
			watchdogInterval: 10 * time.Second,
			expected:         defaultInterval, // 10/2 = 5s, which is > 1s default
		},
		{
			name:             "Watchdog enabled, interval small",
			watchdogEnabled:  true,
			watchdogInterval: 1 * time.Second,
			expected:         500 * time.Millisecond, // 1/2 = 500ms, which is < 1s default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logic.CalculateTickerInterval(tt.watchdogEnabled, tt.watchdogInterval, defaultInterval)
			if got != tt.expected {
				t.Errorf("CalculateTickerInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldWarnInactivity(t *testing.T) {
	timeout := 10 * time.Second
	logic := NewBridgeLogic(timeout)
	now := time.Now()

	lastActivity := now.Add(-5 * time.Second)
	lastWarning := now.Add(-20 * time.Second)

	// Case 1: Active recently
	should, _ := logic.ShouldWarnInactivity(lastActivity, lastWarning, now)
	if should {
		t.Error("Should NOT warn when active recently")
	}

	// Case 2: Inactive, but warned recently
	lastActivity = now.Add(-15 * time.Second)
	lastWarning = now.Add(-5 * time.Second)
	should, _ = logic.ShouldWarnInactivity(lastActivity, lastWarning, now)
	if should {
		t.Error("Should NOT warn when warned recently")
	}

	// Case 3: Inactive and not warned recently
	lastActivity = now.Add(-15 * time.Second)
	lastWarning = now.Add(-20 * time.Second)
	should, newWarning := logic.ShouldWarnInactivity(lastActivity, lastWarning, now)
	if !should {
		t.Error("Should warn when inactive and not warned recently")
	}
	if newWarning != now {
		t.Errorf("Expected new warning time to be %v, got %v", now, newWarning)
	}
}

func TestCalculateBackoff(t *testing.T) {
	logic := NewBridgeLogic(60 * time.Second)
	max := 64 * time.Second

	tests := []struct {
		current  time.Duration
		expected time.Duration
	}{
		{1 * time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{32 * time.Second, 64 * time.Second},
		{64 * time.Second, 64 * time.Second},
	}

	for _, tt := range tests {
		got := logic.CalculateBackoff(tt.current, max)
		if got != tt.expected {
			t.Errorf("CalculateBackoff(%v) = %v, want %v", tt.current, got, tt.expected)
		}
	}
}

func TestShouldLogSerialOpen(t *testing.T) {
	logic := NewBridgeLogic(60 * time.Second)

	tests := []struct {
		firstAttempt bool
		backoff      time.Duration
		expected     bool
	}{
		{true, 1 * time.Second, true},
		{false, 1 * time.Second, false},
		{false, 2 * time.Second, true},
		{true, 2 * time.Second, true},
	}

	for _, tt := range tests {
		got := logic.ShouldLogSerialOpen(tt.firstAttempt, tt.backoff)
		if got != tt.expected {
			t.Errorf("ShouldLogSerialOpen(%v, %v) = %v, want %v", tt.firstAttempt, tt.backoff, got, tt.expected)
		}
	}
}

func TestMergeConfigs(t *testing.T) {
	logic := NewBridgeLogic(60 * time.Second)

	base := config.Config{Device: "base", Broker: "base-broker", QoS: intPtr(0)}
	file := config.Config{Device: "file", Broker: "file-broker"}
	cli := config.Config{Device: "cli", QoS: intPtr(1)}

	// 1. File only overrides base (file has nil QoS, so base QoS preserved)
	merged := logic.MergeConfigs(base, file, config.Config{})
	if merged.Device != "file" || merged.Broker != "file-broker" || *merged.QoS != 0 {
		t.Errorf("MergeConfigs (file only) failed: %+v", merged)
	}

	// 2. CLI overrides both
	merged = logic.MergeConfigs(base, file, cli)
	if merged.Device != "cli" || merged.Broker != "file-broker" || *merged.QoS != 1 {
		t.Errorf("MergeConfigs (full) failed: %+v", merged)
	}

	// 3. Explicit QoS 0 in override should work
	cliQoS0 := config.Config{QoS: intPtr(0)}
	baseQoS2 := config.Config{Device: "base", QoS: intPtr(2)}
	merged = logic.MergeConfigs(baseQoS2, cliQoS0)
	if *merged.QoS != 0 {
		t.Errorf("MergeConfigs (explicit QoS 0) failed: QoS = %d, want 0", *merged.QoS)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0h 0m 0s"},
		{59 * time.Second, "0h 0m 59s"},
		{1 * time.Minute, "0h 1m 0s"},
		{1*time.Hour + 30*time.Minute + 45*time.Second, "1h 30m 45s"},
		{23*time.Hour + 59*time.Minute + 42*time.Second, "23h 59m 42s"},
		{100*time.Hour + 5*time.Minute + 3*time.Second, "100h 5m 3s"},
		// Test rounding
		{1*time.Hour + 30*time.Minute + 45*time.Second + 600*time.Millisecond, "1h 30m 46s"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestUpdateFrameSizeMinMax(t *testing.T) {
	b := &Bridge{}
	b.metrics.frameSizeMin.Store(math.MaxUint64)
	b.metrics.frameSizeMax.Store(0)

	// First frame sets both min and max
	b.updateFrameSizeMinMax(100)
	if b.metrics.frameSizeMin.Load() != 100 {
		t.Errorf("frameSizeMin = %d, want 100", b.metrics.frameSizeMin.Load())
	}
	if b.metrics.frameSizeMax.Load() != 100 {
		t.Errorf("frameSizeMax = %d, want 100", b.metrics.frameSizeMax.Load())
	}

	// Smaller frame updates only min
	b.updateFrameSizeMinMax(50)
	if b.metrics.frameSizeMin.Load() != 50 {
		t.Errorf("frameSizeMin = %d, want 50", b.metrics.frameSizeMin.Load())
	}
	if b.metrics.frameSizeMax.Load() != 100 {
		t.Errorf("frameSizeMax = %d, want 100", b.metrics.frameSizeMax.Load())
	}

	// Larger frame updates only max
	b.updateFrameSizeMinMax(200)
	if b.metrics.frameSizeMin.Load() != 50 {
		t.Errorf("frameSizeMin = %d, want 50", b.metrics.frameSizeMin.Load())
	}
	if b.metrics.frameSizeMax.Load() != 200 {
		t.Errorf("frameSizeMax = %d, want 200", b.metrics.frameSizeMax.Load())
	}

	// Middle value changes nothing
	b.updateFrameSizeMinMax(125)
	if b.metrics.frameSizeMin.Load() != 50 {
		t.Errorf("frameSizeMin = %d, want 50", b.metrics.frameSizeMin.Load())
	}
	if b.metrics.frameSizeMax.Load() != 200 {
		t.Errorf("frameSizeMax = %d, want 200", b.metrics.frameSizeMax.Load())
	}
}

func TestMetricsInitialization(t *testing.T) {
	b := &Bridge{}
	now := time.Now()
	b.metrics.startTime = now
	b.metrics.frameSizeMin.Store(math.MaxUint64)
	b.lastMetricsDump = now

	if b.metrics.framesPublished.Load() != 0 {
		t.Error("framesPublished should start at 0")
	}
	if b.metrics.parseErrors.Load() != 0 {
		t.Error("parseErrors should start at 0")
	}
	if b.metrics.bytesReceived.Load() != 0 {
		t.Error("bytesReceived should start at 0")
	}
	if b.metrics.serialReconnects.Load() != 0 {
		t.Error("serialReconnects should start at 0")
	}
	if b.metrics.mqttPublishFails.Load() != 0 {
		t.Error("mqttPublishFails should start at 0")
	}
	if b.metrics.frameSizeMin.Load() != math.MaxUint64 {
		t.Error("frameSizeMin should start at MaxUint64")
	}
}

func TestCheckMetricsDump(t *testing.T) {
	b := &Bridge{}
	b.metrics.startTime = time.Now()
	b.metrics.frameSizeMin.Store(math.MaxUint64)

	// Set lastMetricsDump to now - should not trigger dump
	b.lastMetricsDump = time.Now()
	oldDumpTime := b.lastMetricsDump
	b.checkMetricsDump()
	if b.lastMetricsDump != oldDumpTime {
		t.Error("checkMetricsDump should not update lastMetricsDump when interval not reached")
	}

	// Set lastMetricsDump to 25 hours ago - should trigger dump
	b.lastMetricsDump = time.Now().Add(-25 * time.Hour)
	b.checkMetricsDump()
	if time.Since(b.lastMetricsDump) > 1*time.Second {
		t.Error("checkMetricsDump should update lastMetricsDump after 24h interval")
	}
}

// mockReader implements io.Reader for testing
type mockReader struct {
	data   []byte
	offset int
}

func (m *mockReader) Read(p []byte) (int, error) {
	if m.offset >= len(m.data) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func TestMetricsBytesReceived(t *testing.T) {
	b := &Bridge{
		parser: parser.NewFrameParser(),
	}
	b.metrics.frameSizeMin.Store(math.MaxUint64)

	// Create a mock reader with some data (not a valid frame, just bytes)
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	reader := &mockReader{data: data}

	// Create a context that we'll use
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run handleSerialConnection - it will read until EOF
	_ = b.handleSerialConnection(ctx, reader)

	if b.metrics.bytesReceived.Load() != uint64(len(data)) {
		t.Errorf("bytesReceived = %d, want %d", b.metrics.bytesReceived.Load(), len(data))
	}
}

func TestMetricsParseErrors(t *testing.T) {
	b := &Bridge{
		parser: parser.NewFrameParser(),
	}
	b.metrics.frameSizeMin.Store(math.MaxUint64)

	// Send bytes that will cause a parse error:
	// Length field 1 (0x0001) results in total size = 3, which is below MinPacketSize (10)
	data := []byte{0x7E, 0x00, 0x01}
	reader := &mockReader{data: data}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = b.handleSerialConnection(ctx, reader)

	if b.metrics.parseErrors.Load() == 0 {
		t.Error("parseErrors should be > 0 after invalid frame data")
	}
}

// mockMQTTToken implements mqtt.Token for testing
type mockMQTTToken struct {
	err error
}

func (m *mockMQTTToken) Wait() bool                     { return true }
func (m *mockMQTTToken) WaitTimeout(time.Duration) bool { return true }
func (m *mockMQTTToken) Done() <-chan struct{}          { ch := make(chan struct{}); close(ch); return ch }
func (m *mockMQTTToken) Error() error                   { return m.err }

// mockMQTTClient implements mqtt.Client for testing
type mockMQTTClient struct {
	publishErr   error
	publishCount int
}

func (m *mockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	m.publishCount++
	return &mockMQTTToken{err: m.publishErr}
}

// Implement remaining mqtt.Client interface methods as no-ops
func (m *mockMQTTClient) IsConnected() bool       { return true }
func (m *mockMQTTClient) IsConnectionOpen() bool  { return true }
func (m *mockMQTTClient) Connect() mqtt.Token     { return &mockMQTTToken{} }
func (m *mockMQTTClient) Disconnect(quiesce uint) {}
func (m *mockMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mockMQTTToken{}
}
func (m *mockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mockMQTTToken{}
}
func (m *mockMQTTClient) Unsubscribe(topics ...string) mqtt.Token             { return &mockMQTTToken{} }
func (m *mockMQTTClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (m *mockMQTTClient) OptionsReader() mqtt.ClientOptionsReader             { return mqtt.ClientOptionsReader{} }

func TestMetricsFramesPublished(t *testing.T) {
	mockClient := &mockMQTTClient{}
	b := &Bridge{
		parser: parser.NewFrameParser(),
		mqtt:   mockClient,
		config: config.Config{Topic: "test/topic", QoS: intPtr(0)},
	}
	b.metrics.frameSizeMin.Store(math.MaxUint64)

	// Build a valid frame: 0x7E + length bytes + payload + 0x7E
	// Length = 8, so total size = 8 + 2 = 10 bytes (minimum valid)
	// Length field: ((byte1 & 0x0F) << 8) | byte2 = 8
	// Frame: delimiter(1) + lengthMSB(1) + lengthLSB(1) + payload(6) + delimiter(1) = 10
	frame := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}

	reader := &mockReader{data: frame}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = b.handleSerialConnection(ctx, reader)

	if b.metrics.framesPublished.Load() != 1 {
		t.Errorf("framesPublished = %d, want 1", b.metrics.framesPublished.Load())
	}
	if mockClient.publishCount != 1 {
		t.Errorf("MQTT publishCount = %d, want 1", mockClient.publishCount)
	}
	if b.metrics.frameSizeTotal.Load() != 10 {
		t.Errorf("frameSizeTotal = %d, want 10", b.metrics.frameSizeTotal.Load())
	}
	if b.metrics.frameSizeMin.Load() != 10 {
		t.Errorf("frameSizeMin = %d, want 10", b.metrics.frameSizeMin.Load())
	}
	if b.metrics.frameSizeMax.Load() != 10 {
		t.Errorf("frameSizeMax = %d, want 10", b.metrics.frameSizeMax.Load())
	}
}

func TestMetricsMQTTPublishFails(t *testing.T) {
	mockClient := &mockMQTTClient{publishErr: io.ErrUnexpectedEOF}
	b := &Bridge{
		parser: parser.NewFrameParser(),
		mqtt:   mockClient,
		config: config.Config{Topic: "test/topic", QoS: intPtr(0)},
	}
	b.metrics.frameSizeMin.Store(math.MaxUint64)

	// Build a valid frame (same as TestMetricsFramesPublished)
	frame := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}

	reader := &mockReader{data: frame}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = b.handleSerialConnection(ctx, reader)

	if b.metrics.mqttPublishFails.Load() != 1 {
		t.Errorf("mqttPublishFails = %d, want 1", b.metrics.mqttPublishFails.Load())
	}
	// Frame should NOT be counted as published when publish fails
	if b.metrics.framesPublished.Load() != 0 {
		t.Errorf("framesPublished = %d, want 0 (publish failed)", b.metrics.framesPublished.Load())
	}
}
