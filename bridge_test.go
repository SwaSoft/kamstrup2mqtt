package main

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// fakeToken is a completed mqtt.Token.
type fakeToken struct {
	mqtt.Token
	err error
}

func (t *fakeToken) Wait() bool                     { return true }
func (t *fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t *fakeToken) Error() error                   { return t.err }

// fakeClient records published payloads; other Client methods are unused.
type fakeClient struct {
	mqtt.Client
	mu        sync.Mutex
	topics    []string
	qos       []byte
	payloads  [][]byte
	publishEr error
}

func (c *fakeClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.topics = append(c.topics, topic)
	c.qos = append(c.qos, qos)
	c.payloads = append(c.payloads, append([]byte(nil), payload.([]byte)...))
	return &fakeToken{err: c.publishEr}
}

func (c *fakeClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.payloads)
}

func testBridge(client mqtt.Client) *Bridge {
	cfg := DefaultConfig("host", 1)
	cfg.Device = "/nonexistent/kamstrup-test-device"
	cfg.Topic = "test/topic"
	cfg.QoS = intPtr(1)
	return NewBridge(cfg, client)
}

func TestNewBridge_Initial(t *testing.T) {
	fc := &fakeClient{}
	b := testBridge(fc)
	if b.mqtt != fc || b.parser == nil || b.logic == nil {
		t.Fatal("bridge fields not initialised")
	}
	if b.config.Topic != "test/topic" {
		t.Errorf("topic = %q", b.config.Topic)
	}
	if b.metrics.frameSizeMin.Load() != math.MaxUint64 {
		t.Error("frameSizeMin should start at MaxUint64")
	}
	if b.metrics.startTime.IsZero() || b.lastMetricsDump.IsZero() || b.lastActivity.Load() == 0 {
		t.Error("timestamps not initialised")
	}
	if b.watchdogEnabled != (b.watchdogInterval > 0) {
		t.Error("watchdog flag inconsistent with interval")
	}
}

func TestHandleSerialConnection_PublishesFramesAndMetrics(t *testing.T) {
	fc := &fakeClient{}
	b := testBridge(fc)
	f1, f2 := mkFrame(MinPacketSize), mkFrame(MinPacketSize+10)
	data := append(append([]byte{}, f1...), f2...)

	if err := b.handleSerialConnection(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.count() != 2 {
		t.Fatalf("published %d frames, want 2", fc.count())
	}
	if fc.topics[0] != "test/topic" || fc.qos[0] != 1 {
		t.Errorf("topic/qos = %q/%d", fc.topics[0], fc.qos[0])
	}
	m := &b.metrics
	if m.framesPublished.Load() != 2 {
		t.Errorf("framesPublished = %d", m.framesPublished.Load())
	}
	if m.bytesReceived.Load() != uint64(len(data)) {
		t.Errorf("bytesReceived = %d", m.bytesReceived.Load())
	}
	if m.frameSizeMin.Load() != uint64(len(f1)) || m.frameSizeMax.Load() != uint64(len(f2)) {
		t.Errorf("min/max = %d/%d", m.frameSizeMin.Load(), m.frameSizeMax.Load())
	}
	if m.frameSizeTotal.Load() != uint64(len(f1)+len(f2)) {
		t.Errorf("total = %d", m.frameSizeTotal.Load())
	}
	b.logMetrics() // with frames
}

func TestHandleSerialConnection_ParseErrorsAndPublishFailures(t *testing.T) {
	fc := &fakeClient{publishEr: errors.New("boom")}
	b := testBridge(fc)
	bad := mkFrame(MinPacketSize)
	bad[len(bad)-1] = 0x00 // malformed end
	data := append(append([]byte{}, bad...), mkFrame(MinPacketSize)...)

	if err := b.handleSerialConnection(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if b.metrics.parseErrors.Load() == 0 {
		t.Error("expected parse errors")
	}
	if b.metrics.mqttPublishFails.Load() != 1 {
		t.Errorf("publish fails = %d", b.metrics.mqttPublishFails.Load())
	}
	if b.metrics.framesPublished.Load() != 0 {
		t.Error("failed publish must not count as published")
	}
	b.logMetrics() // no frames
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestHandleSerialConnection_ReadErrorAndCancel(t *testing.T) {
	b := testBridge(&fakeClient{})
	want := errors.New("read failed")
	if err := b.handleSerialConnection(context.Background(), errReader{want}); err != want {
		t.Errorf("err = %v, want %v", err, want)
	}

	// EOF mid-frame is not an error.
	b.parser.Reset()
	if err := b.handleSerialConnection(context.Background(), bytes.NewReader(mkFrame(MinPacketSize)[:5])); err != nil {
		t.Errorf("EOF err = %v", err)
	}

	// Cancelled context returns nil, even with partial frame in the parser.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.handleSerialConnection(ctx, errReader{want}); err != nil {
		t.Errorf("cancel err = %v", err)
	}
}

func TestCheckInactivityAndMetricsDump(t *testing.T) {
	b := testBridge(&fakeClient{})
	b.checkInactivity()
	if !b.lastInactivityWarning.IsZero() {
		t.Error("no warning expected for recent activity")
	}
	b.lastActivity.Store(time.Now().Add(-2 * InactivityTimeout).UnixNano())
	b.checkInactivity()
	if b.lastInactivityWarning.IsZero() {
		t.Error("warning expected after inactivity")
	}

	before := b.lastMetricsDump
	b.checkMetricsDump()
	if !b.lastMetricsDump.Equal(before) {
		t.Error("dump should not happen yet")
	}
	b.lastMetricsDump = time.Now().Add(-2 * MetricsDumpInterval)
	b.checkMetricsDump()
	if time.Since(b.lastMetricsDump) > time.Minute {
		t.Error("dump timestamp not refreshed")
	}
}

func TestBuildTLSConfig(t *testing.T) {
	cfg := DefaultConfig("host", 1)
	cfg.TLSInsecure = boolPtr(true)
	tc, err := buildTLSConfig(cfg)
	if err != nil || !tc.InsecureSkipVerify {
		t.Errorf("insecure: %v %v", tc, err)
	}
	cfg = DefaultConfig("host", 1)
	cfg.TLSCa = "/nonexistent/ca.pem"
	if _, err := buildTLSConfig(cfg); err == nil {
		t.Error("expected error for missing CA")
	}
	ca := t.TempDir() + "/ca.pem"
	if err := os.WriteFile(ca, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.TLSCa = ca
	if _, err := buildTLSConfig(cfg); err == nil {
		t.Error("expected error for invalid CA")
	}
	cfg = DefaultConfig("host", 1)
	cfg.TLSCert, cfg.TLSKey = "/nonexistent/c", "/nonexistent/k"
	if _, err := buildTLSConfig(cfg); err == nil {
		t.Error("expected error for missing keypair")
	}
}

func TestStart_SignalShutdown(t *testing.T) {
	b := testBridge(&fakeClient{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)

	done := make(chan struct{})
	go func() { b.Start(ctx, cancel, sig); close(done) }()

	time.Sleep(100 * time.Millisecond) // let the serial loop attempt an open
	sig <- syscall.SIGTERM

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after signal")
	}
	if ctx.Err() == nil {
		t.Error("context should be cancelled on signal")
	}
	if b.metrics.serialReconnects.Load() < 1 {
		t.Error("expected at least one failed serial open counted")
	}
}

func TestStart_ContextCancelStopsUnexpectedly(t *testing.T) {
	b := testBridge(&fakeClient{})
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)

	done := make(chan struct{})
	go func() { b.Start(ctx, cancel, sig); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel() // serial loop exits -> Start returns

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestRunSerialLoop_BackoffCancel(t *testing.T) {
	b := testBridge(&fakeClient{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go b.runSerialLoop(ctx, done)

	deadline := time.Now().Add(2 * time.Second)
	for b.metrics.serialReconnects.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if b.metrics.serialReconnects.Load() < 1 {
		t.Fatal("serial open failure not counted")
	}
	cancel() // interrupts the backoff wait
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serial loop did not stop during backoff")
	}

	// Already-cancelled context: loop returns immediately without opening.
	b2 := testBridge(&fakeClient{})
	done2 := make(chan struct{})
	go b2.runSerialLoop(ctx, done2)
	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("loop should exit immediately")
	}
	if b2.metrics.serialReconnects.Load() != 0 {
		t.Error("no open should be attempted")
	}
}
