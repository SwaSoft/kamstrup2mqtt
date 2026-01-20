package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tarm/serial"
)

// Frame constants
const (
	Delimiter         = 0x7E
	MinPacketSize     = 10
	MaxPacketSize     = 4097
	InactivityTimeout = 60 * time.Second
)

// Config holds all application settings.
type Config struct {
	Device   string `json:"device"`
	Baud     int    `json:"baud"`
	Broker   string `json:"broker"`
	Topic    string `json:"topic"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() Config {
	return Config{
		Device: "/dev/ttyUSB0",
		Baud:   2400,
		Broker: "tcp://localhost:1883",
		Topic:  "serial/bridge",
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

// Reset clears the current parsing state.
func (p *FrameParser) Reset() {
	p.frameStarted = false
	p.frameBuffer = nil
	p.byteCounter = 0
	p.expectedPacketSize = 0
}

// ProcessByte handles an incoming byte and returns a complete frame if one is found.
func (p *FrameParser) ProcessByte(b byte) ([]byte, error) {
	if !p.frameStarted {
		if b == Delimiter {
			p.frameStarted = true
			p.frameBuffer = []byte{b}
			p.byteCounter = 1
			p.expectedPacketSize = 0
		}
		return nil, nil
	}

	// Frame Accumulation
	p.frameBuffer = append(p.frameBuffer, b)
	p.byteCounter++

	// Check for early start delimiter (Resynchronization)
	if b == Delimiter {
		// If we are at the expected end, handle naturally
		if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
			frame := p.frameBuffer
			p.Reset()
			return frame, nil
		}
		// Otherwise, it's an early/unexpected delimiter, treat as start of new frame
		oldCounter := p.byteCounter
		p.Reset()
		p.frameStarted = true
		p.frameBuffer = []byte{b}
		p.byteCounter = 1
		return nil, fmt.Errorf("malformed frame: unexpected delimiter at index %d, starting new frame", oldCounter-1)
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

	// Frame Completion
	if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
		// If we reach here, b != Delimiter because we checked for it at the start of the function
		p.Reset()
		return nil, fmt.Errorf("malformed frame: last byte is 0x%02X not 0x%02X", b, Delimiter)
	}

	return nil, nil
}

func main() {
	// Flags definition
	configPath := flag.String("config", "", "Path to configuration file (JSON)")
	device := flag.String("device", "", "Serial device path")
	baud := flag.Int("baud", 0, "Serial baud rate")
	broker := flag.String("broker", "", "MQTT broker address")
	topic := flag.String("topic", "", "MQTT topic")
	user := flag.String("user", "", "MQTT username")
	pass := flag.String("pass", "", "MQTT password")
	passFile := flag.String("passfile", "", "Path to file containing MQTT password (overrides -pass)")
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

	// MQTT Setup
	opts := mqtt.NewClientOptions().AddBroker(cfg.Broker).SetUsername(cfg.User).SetPassword(cfg.Password)
	opts.SetClientID("serial2mqtt")
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(300 * time.Second)
	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("Connected to MQTT broker at %s", cfg.Broker)
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("Lost connection to MQTT broker: %v", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Fatal error connecting to MQTT: %v", token.Error())
	}
	defer client.Disconnect(250)

	// Interrupt handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Parser and Activity
	parser := &FrameParser{}
	lastActivity := time.Now()

	// Serial main loop with reconnection
	errChan := make(chan error, 1)
	go func() {
		backoff := 1 * time.Second
		maxBackoff := 60 * time.Second
		firstAttempt := true

		for {
			// Only log opening if it's the first time or if we've been failing
			if firstAttempt || backoff > 1*time.Second {
				log.Printf("Opening serial port %s at %d baud\n", cfg.Device, cfg.Baud)
			}

			c := &serial.Config{Name: cfg.Device, Baud: cfg.Baud, ReadTimeout: 15 * time.Second}
			s, err := serial.OpenPort(c)
			if err != nil {
				log.Printf("Error opening serial port: %v. Retrying in %v...", err, backoff)
				select {
				case <-sigChan:
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

			buf := make([]byte, 1)
			for {
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
					break
				}

				if n == 0 {
					if parser.frameStarted {
						parser.Reset()
					}
					continue
				}

				frame, err := parser.ProcessByte(buf[0])
				if err != nil {
					if !firstErrorSilent {
						log.Printf("Warning: %v, discarding", err)
					}
					firstErrorSilent = false
					continue
				}

				if frame != nil {
					firstErrorSilent = false
					token := client.Publish(cfg.Topic, 0, false, frame)
					if token.Wait() && token.Error() != nil {
						log.Printf("Error publishing to MQTT: %v", token.Error())
					} else {
						if firstFrameLogNeeded {
							log.Printf("First valid frame published to %s: %d bytes (operational)", cfg.Topic, len(frame))
							firstFrameLogNeeded = false
						}
						lastActivity = time.Now()
					}
				}
			}

			// Small delay before reconnection
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Ticker for inactivity warning
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			log.Println("Shutting down...")
			return
		case err := <-errChan:
			log.Fatalf("Fatal error: %v", err)
		case <-ticker.C:
			if time.Since(lastActivity) > InactivityTimeout {
				log.Println("Warning: No valid frame published for more than 60 seconds")
				lastActivity = time.Now() // Reset timer to repeat once per minute
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
