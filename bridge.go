package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tarm/serial"
)

// Bridge-related constants
const (
	InactivityTimeout    = 60 * time.Second
	MQTTConnTimeout      = 30 * time.Second
	SerialReadBufSize    = 256
	MetricsDumpInterval  = 24 * time.Hour
)

// Metrics tracks operational statistics for the bridge.
type Metrics struct {
	framesPublished  atomic.Uint64
	parseErrors      atomic.Uint64
	bytesReceived    atomic.Uint64
	serialReconnects atomic.Uint64
	mqttPublishFails atomic.Uint64
	frameSizeMin     atomic.Uint64
	frameSizeMax     atomic.Uint64
	frameSizeTotal   atomic.Uint64
	startTime        time.Time
}

// Bridge encapsulates the serial-to-MQTT bridging logic (Imperative Shell).
type Bridge struct {
	config                Config
	mqtt                  mqtt.Client
	parser                *FrameParser
	logic                 *BridgeLogic
	lastActivity          atomic.Int64
	lastInactivityWarning time.Time
	watchdogEnabled       bool
	watchdogInterval      time.Duration
	metrics               Metrics
	lastMetricsDump       time.Time
}

// NewBridge creates a new Bridge instance.
func NewBridge(cfg Config, client mqtt.Client) *Bridge {
	now := time.Now()
	b := &Bridge{
		config:          cfg,
		mqtt:            client,
		parser:          NewFrameParser(),
		logic:           NewBridgeLogic(InactivityTimeout),
		lastMetricsDump: now,
	}
	b.lastActivity.Store(now.UnixNano())
	b.metrics.startTime = now
	b.metrics.frameSizeMin.Store(math.MaxUint64)

	wi, err := daemon.SdWatchdogEnabled(false)
	b.watchdogEnabled = err == nil && wi > 0
	b.watchdogInterval = wi

	return b
}

// Start runs the bridge until either the context is canceled or a signal is received on sigChan.
func (b *Bridge) Start(ctx context.Context, sigChan chan os.Signal) {
	daemon.SdNotify(false, daemon.SdNotifyReady)

	serialDone := make(chan struct{})
	go b.runSerialLoop(ctx, serialDone)

	tickerInterval := b.logic.CalculateTickerInterval(b.watchdogEnabled, b.watchdogInterval, 1*time.Second)
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case s := <-sigChan:
			log.Printf("Received signal %v, shutting down...", s)
			b.logMetrics()
			daemon.SdNotify(false, daemon.SdNotifyStopping)
			return

		case <-ticker.C:
			if b.watchdogEnabled {
				daemon.SdNotify(false, daemon.SdNotifyWatchdog)
			}
			b.checkInactivity()
			b.checkMetricsDump()

		case <-serialDone:
			log.Println("Serial loop terminated unexpectedly")
			return
		}
	}
}

func (b *Bridge) runSerialLoop(ctx context.Context, done chan struct{}) {
	defer close(done)

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	firstAttempt := true

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if b.logic.ShouldLogSerialOpen(firstAttempt, backoff) {
			log.Printf("Opening serial port %s", b.config.Device)
		}

		sc := &serial.Config{Name: b.config.Device, Baud: 2400, ReadTimeout: 15 * time.Second}
		s, err := serial.OpenPort(sc)
		if err != nil {
			b.metrics.serialReconnects.Add(1)
			log.Printf("Error opening serial port: %v. Retrying in %v...", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff = b.logic.CalculateBackoff(backoff, maxBackoff)
				continue
			}
		}

		if backoff > 1*time.Second {
			log.Println("Serial port re-opened successfully")
		}

		backoff = 1 * time.Second
		firstAttempt = false
		b.parser.Reset()

		if err := b.handleSerialConnection(ctx, s); err != nil {
			log.Printf("Serial connection error: %v", err)
		}
		s.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (b *Bridge) handleSerialConnection(ctx context.Context, s io.Reader) error {
	buf := make([]byte, SerialReadBufSize)
	firstErrorSilent := true
	firstFrameLogNeeded := true

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := s.Read(buf)
		if n > 0 {
			b.metrics.bytesReceived.Add(uint64(n))
		}
		if err != nil {
			if err == io.EOF {
				if b.parser.frameStarted {
					log.Println("Warning: Serial timeout/EOF inside frame, discarding partial data")
				}
				return nil
			}
			return err
		}

		for i := 0; i < n; i++ {
			frame, err := b.parser.ProcessByte(buf[i])
			if err != nil {
				b.metrics.parseErrors.Add(1)
				if !firstErrorSilent {
					log.Printf("Warning: %v, discarding", err)
				}
				firstErrorSilent = false
				continue
			}

			if frame != nil {
				firstErrorSilent = false
				token := b.mqtt.Publish(b.config.Topic, byte(*b.config.QoS), false, frame)
				if token.Wait() && token.Error() != nil {
					b.metrics.mqttPublishFails.Add(1)
					log.Printf("Error publishing to MQTT: %v", token.Error())
				} else {
					frameSize := uint64(len(frame))
					b.metrics.framesPublished.Add(1)
					b.metrics.frameSizeTotal.Add(frameSize)
					b.updateFrameSizeMinMax(frameSize)
					if firstFrameLogNeeded {
						log.Printf("First valid frame published to %s: %d bytes (operational)", b.config.Topic, len(frame))
						firstFrameLogNeeded = false
					}
					b.lastActivity.Store(time.Now().UnixNano())
				}
			}
		}
	}
}

func (b *Bridge) checkInactivity() {
	now := time.Now()
	lastActivity := time.Unix(0, b.lastActivity.Load())

	shouldWarn, nextWarning := b.logic.ShouldWarnInactivity(lastActivity, b.lastInactivityWarning, now)
	if shouldWarn {
		log.Printf("Warning: No valid frame published to %s for more than %v", b.config.Topic, InactivityTimeout)
		b.lastInactivityWarning = nextWarning
	}
}

func (b *Bridge) updateFrameSizeMinMax(size uint64) {
	for {
		oldMin := b.metrics.frameSizeMin.Load()
		if size >= oldMin || b.metrics.frameSizeMin.CompareAndSwap(oldMin, size) {
			break
		}
	}
	for {
		oldMax := b.metrics.frameSizeMax.Load()
		if size <= oldMax || b.metrics.frameSizeMax.CompareAndSwap(oldMax, size) {
			break
		}
	}
}

func (b *Bridge) checkMetricsDump() {
	if time.Since(b.lastMetricsDump) >= MetricsDumpInterval {
		b.logMetrics()
		b.lastMetricsDump = time.Now()
	}
}

func (b *Bridge) logMetrics() {
	uptime := time.Since(b.metrics.startTime)
	framesPublished := b.metrics.framesPublished.Load()
	parseErrors := b.metrics.parseErrors.Load()
	bytesReceived := b.metrics.bytesReceived.Load()
	serialReconnects := b.metrics.serialReconnects.Load()
	mqttPublishFails := b.metrics.mqttPublishFails.Load()
	frameSizeMin := b.metrics.frameSizeMin.Load()
	frameSizeMax := b.metrics.frameSizeMax.Load()
	frameSizeTotal := b.metrics.frameSizeTotal.Load()

	var frameSizeStr string
	if framesPublished > 0 {
		avg := frameSizeTotal / framesPublished
		if frameSizeMin == math.MaxUint64 {
			frameSizeMin = 0
		}
		frameSizeStr = fmt.Sprintf("min=%d max=%d avg=%d", frameSizeMin, frameSizeMax, avg)
	} else {
		frameSizeStr = "no frames"
	}

	log.Printf("Daily metrics report:\n"+
		"  Uptime: %s\n"+
		"  Frames published: %d\n"+
		"  Parse errors: %d\n"+
		"  Bytes received: %d\n"+
		"  Serial reconnections: %d\n"+
		"  MQTT publish failures: %d\n"+
		"  Frame sizes: %s",
		formatDuration(uptime),
		framesPublished,
		parseErrors,
		bytesReceived,
		serialReconnects,
		mqttPublishFails,
		frameSizeStr)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

func initMQTT(cfg Config) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(cfg.Broker).SetUsername(cfg.User).SetPassword(cfg.Password)
	opts.SetClientID(cfg.ClientID)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(300 * time.Second)
	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("Connected to MQTT broker at %s", cfg.Broker)
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("Lost connection to MQTT broker: %v", err)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(MQTTConnTimeout) {
		return nil, fmt.Errorf("MQTT connection timeout after %v", MQTTConnTimeout)
	}
	if token.Error() != nil {
		return nil, token.Error()
	}
	return client, nil
}
