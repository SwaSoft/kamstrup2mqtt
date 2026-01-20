package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tarm/serial"
)

// Frame constants
const (
	Delimiter         = 0x7E
	MinPacketSize     = 10
	MaxPacketSize     = 4097
	InactivityTimeout = 60 * time.Second
	MQTTConnTimeout   = 30 * time.Second
	SerialReadBufSize = 256
)

// Config holds all application settings.
type Config struct {
	Device   string `json:"device"`
	Baud     int    `json:"baud"`
	Broker   string `json:"broker"`
	Topic    string `json:"topic"`
	User     string `json:"user"`
	Password string `json:"password"`
	ClientID string `json:"client_id"`
	QoS      int    `json:"qos"`
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() Config {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return Config{
		Device:   "/dev/ttyUSB0",
		Baud:     2400,
		Broker:   "tcp://localhost:1883",
		Topic:    "serial/bridge",
		ClientID: fmt.Sprintf("serial2mqtt-%s-%d", hostname, os.Getpid()),
		QoS:      0,
	}
}

// LoadConfig loads configuration from a JSON file.
func LoadConfig(path string) (Config, error) {
	var config Config
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}

// FrameParser handles the state machine for parsing serial frames.
type FrameParser struct {
	frameStarted       bool
	frameBuffer        []byte
	byteCounter        int
	expectedPacketSize int
}

// NewFrameParser creates a new FrameParser with pre-allocated buffer.
func NewFrameParser() *FrameParser {
	return &FrameParser{
		frameBuffer: make([]byte, 0, MaxPacketSize),
	}
}

// Reset clears the current parsing state.
func (p *FrameParser) Reset() {
	p.frameStarted = false
	p.frameBuffer = p.frameBuffer[:0] // Keep capacity, reset length
	p.byteCounter = 0
	p.expectedPacketSize = 0
}

// ProcessByte handles an incoming byte and returns a complete frame if one is found.
func (p *FrameParser) ProcessByte(b byte) ([]byte, error) {
	if !p.frameStarted {
		if b == Delimiter {
			p.frameStarted = true
			p.frameBuffer = append(p.frameBuffer[:0], b)
			p.byteCounter = 1
			p.expectedPacketSize = 0
		}
		return nil, nil
	}

	// Frame Accumulation
	p.frameBuffer = append(p.frameBuffer, b)
	p.byteCounter++

	// Handle delimiters
	if b == Delimiter {
		// Valid end?
		if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
			// Return a copy to avoid buffer reuse issues
			frame := make([]byte, len(p.frameBuffer))
			copy(frame, p.frameBuffer)
			p.Reset()
			return frame, nil
		}

		// Redundant start or early sync (only before length is reliable)
		if p.byteCounter < 3 {
			p.Reset()
			p.frameStarted = true
			p.frameBuffer = append(p.frameBuffer[:0], b)
			p.byteCounter = 1
			return nil, nil // Silent resync for redundant starts
		}

		// If byteCounter >= 3 and not at the end, we treat it as payload data.
		// Delimiters are valid in binary payloads as long as the length allows.
	}

	// Length Determination
	if p.byteCounter == 3 {
		// Byte 1: length MSB (low nibble), Byte 2: length LSB
		length := (int(p.frameBuffer[1]&0x0F) << 8) | int(p.frameBuffer[2])
		p.expectedPacketSize = length + 2
		if p.expectedPacketSize < MinPacketSize || p.expectedPacketSize > MaxPacketSize {
			actualSize := p.expectedPacketSize
			p.Reset()
			return nil, fmt.Errorf("invalid packet size %d (length=%d)", actualSize, length)
		}
	}

	// Frame Completion (for non-delimiter bytes at the expected end)
	if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
		// If we reach here, b != Delimiter (checked above)
		p.Reset()
		return nil, fmt.Errorf("malformed frame: last byte at index %d is 0x%02X not 0x%02X", p.byteCounter-1, b, Delimiter)
	}

	return nil, nil
}

func main() {
	// Detect if running under systemd (journald handles timestamps)
	if os.Getenv("JOURNAL_STREAM") != "" {
		log.SetFlags(0)
	}

	// Flags definition
	configPath := flag.String("config", "", "Path to configuration file (JSON)")
	device := flag.String("device", "", "Serial device path")
	baud := flag.Int("baud", 0, "Serial baud rate")
	broker := flag.String("broker", "", "MQTT broker address")
	topic := flag.String("topic", "", "MQTT topic")
	user := flag.String("user", "", "MQTT username")
	pass := flag.String("pass", "", "MQTT password")
	passFile := flag.String("passfile", "", "Path to file containing MQTT password (overrides -pass)")
	clientID := flag.String("clientid", "", "MQTT client ID (default: serial2mqtt-<hostname>-<pid>)")
	qos := flag.Int("qos", -1, "MQTT QoS level (0, 1, or 2)")
	flag.Parse()

	// 1. Start with defaults
	cfg := DefaultConfig()

	// 2. Override with config file if provided
	if *configPath != "" {
		fileCfg, err := LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Error loading config file: %v", err)
		}
		// Merge fileCfg into cfg (if values are non-zero/non-empty)
		if fileCfg.Device != "" {
			cfg.Device = fileCfg.Device
		}
		if fileCfg.Baud != 0 {
			cfg.Baud = fileCfg.Baud
		}
		if fileCfg.Broker != "" {
			cfg.Broker = fileCfg.Broker
		}
		if fileCfg.Topic != "" {
			cfg.Topic = fileCfg.Topic
		}
		if fileCfg.User != "" {
			cfg.User = fileCfg.User
		}
		if fileCfg.Password != "" {
			cfg.Password = fileCfg.Password
		}
		if fileCfg.ClientID != "" {
			cfg.ClientID = fileCfg.ClientID
		}
		if fileCfg.QoS != 0 {
			cfg.QoS = fileCfg.QoS
		}
	}

	// 3. Override with command line flags
	if *device != "" {
		cfg.Device = *device
	}
	if *baud != 0 {
		cfg.Baud = *baud
	}
	if *broker != "" {
		cfg.Broker = *broker
	}
	if *topic != "" {
		cfg.Topic = *topic
	}
	if *user != "" {
		cfg.User = *user
	}
	if *pass != "" {
		cfg.Password = *pass
	}
	if *passFile != "" {
		p, err := readPassword(*passFile)
		if err != nil {
			log.Fatalf("Error reading password file: %v", err)
		}
		cfg.Password = p
	}
	if *clientID != "" {
		cfg.ClientID = *clientID
	}
	if *qos >= 0 {
		cfg.QoS = *qos
	}

	// Validate QoS
	if cfg.QoS < 0 || cfg.QoS > 2 {
		log.Fatalf("Invalid QoS level %d (must be 0, 1, or 2)", cfg.QoS)
	}

	// MQTT Setup
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
		log.Fatalf("MQTT connection timeout after %v", MQTTConnTimeout)
	}
	if token.Error() != nil {
		log.Fatalf("Fatal error connecting to MQTT: %v", token.Error())
	}

	// Context for coordinated shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Interrupt handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Notify systemd we're ready
	daemon.SdNotify(false, daemon.SdNotifyReady)

	// Check if watchdog is enabled
	watchdogInterval, err := daemon.SdWatchdogEnabled(false)
	watchdogEnabled := err == nil && watchdogInterval > 0

	// Parser and Activity (atomic for thread safety)
	parser := NewFrameParser()
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())

	// Channel to signal serial goroutine has stopped
	serialDone := make(chan struct{})

	// Serial main loop with reconnection
	go func() {
		defer close(serialDone)

		backoff := 1 * time.Second
		maxBackoff := 60 * time.Second
		firstAttempt := true

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Only log opening if it's the first time or if we've been failing
			if firstAttempt || backoff > 1*time.Second {
				log.Printf("Opening serial port %s at %d baud", cfg.Device, cfg.Baud)
			}

			c := &serial.Config{Name: cfg.Device, Baud: cfg.Baud, ReadTimeout: 15 * time.Second}
			s, err := serial.OpenPort(c)
			if err != nil {
				log.Printf("Error opening serial port: %v. Retrying in %v...", err, backoff)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}
			}

			// If we reached here, port is open. If we were in backoff mode, log recovery.
			if backoff > 1*time.Second {
				log.Println("Serial port re-opened successfully")
			}

			backoff = 1 * time.Second
			firstAttempt = false
			parser.Reset()
			firstErrorSilent := true
			firstFrameLogNeeded := true

			buf := make([]byte, SerialReadBufSize)
			readLoop := true
			for readLoop {
				select {
				case <-ctx.Done():
					s.Close()
					return
				default:
				}

				n, err := s.Read(buf)
				if err != nil {
					s.Close()
					// Suppress EOF noise: don't log if it's a routine timeout (EOF)
					if err != io.EOF {
						log.Printf("Warning: Serial read error: %v", err)
						// Ensure we trigger backoff on real errors
						backoff = 2 * time.Second
					} else {
						// Routine timeout (or port closed)
						// If we were mid-frame, maybe log it
						if parser.frameStarted {
							log.Println("Warning: Serial timeout/EOF inside frame, discarding partial data")
						}
					}
					readLoop = false
					break
				}

				for i := 0; i < n; i++ {
					frame, err := parser.ProcessByte(buf[i])
					if err != nil {
						if !firstErrorSilent {
							log.Printf("Warning: %v, discarding", err)
						}
						firstErrorSilent = false
						continue
					}

					if frame != nil {
						firstErrorSilent = false
						token := client.Publish(cfg.Topic, byte(cfg.QoS), false, frame)
						if token.Wait() && token.Error() != nil {
							log.Printf("Error publishing to MQTT: %v", token.Error())
						} else {
							if firstFrameLogNeeded {
								log.Printf("First valid frame published to %s: %d bytes (operational)", cfg.Topic, len(frame))
								firstFrameLogNeeded = false
							}
							lastActivity.Store(time.Now().UnixNano())
						}
					}
				}
			}

			// Small delay before reconnection
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()

	// Ticker for inactivity warning and watchdog
	tickerInterval := 1 * time.Second
	if watchdogEnabled && watchdogInterval/2 > 0 && watchdogInterval/2 < tickerInterval {
		tickerInterval = watchdogInterval / 2
	}
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	var lastInactivityWarning time.Time

	for {
		select {
		case <-sigChan:
			log.Println("Shutting down...")
			daemon.SdNotify(false, daemon.SdNotifyStopping)
			cancel()

			// Wait for serial goroutine to finish (with timeout)
			select {
			case <-serialDone:
			case <-time.After(5 * time.Second):
				log.Println("Warning: Serial goroutine did not stop in time")
			}

			client.Disconnect(250)
			return

		case <-ticker.C:
			// Watchdog notification
			if watchdogEnabled {
				daemon.SdNotify(false, daemon.SdNotifyWatchdog)
			}

			// Inactivity check
			lastActivityTime := time.Unix(0, lastActivity.Load())
			if time.Since(lastActivityTime) > InactivityTimeout {
				if time.Since(lastInactivityWarning) > InactivityTimeout {
					log.Println("Warning: No valid frame published for more than 60 seconds")
					lastInactivityWarning = time.Now()
				}
			}
		}
	}
}

func readPassword(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
