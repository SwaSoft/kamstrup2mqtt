package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"kamstrup2mqtt/internal/bridge"
	"kamstrup2mqtt/internal/config"
)

// buildConfig merges defaults, the config file, the credentials file and CLI flags
// (in increasing order of precedence) and validates the result.
func buildConfig(configPath, credfilePath, device, broker, topic, user, pass, clientID string, qos int, tlsCa, tlsCert, tlsKey string, tlsInsecure int) (config.Config, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	pid := os.Getpid()

	base := config.DefaultConfig(hostname, pid)

	var fileCfg config.Config
	if configPath != "" {
		var err error
		fileCfg, err = config.LoadConfig(configPath)
		if err != nil {
			return base, fmt.Errorf("error loading config file: %v", err)
		}
	}

	var credCfg config.Config
	if credfilePath != "" {
		credUser, credPass, err := config.LoadCredentials(credfilePath)
		if err != nil {
			return base, fmt.Errorf("error loading credentials: %v", err)
		}
		credCfg.User = credUser
		credCfg.Password = credPass
	}

	cliFlags := config.Config{
		Device:   device,
		Broker:   broker,
		Topic:    topic,
		User:     user,
		Password: pass,
		ClientID: clientID,
		TLSCa:    tlsCa,
		TLSCert:  tlsCert,
		TLSKey:   tlsKey,
	}
	if qos >= 0 {
		cliFlags.QoS = &qos
	}
	if tlsInsecure >= 0 {
		insecure := tlsInsecure == 1
		cliFlags.TLSInsecure = &insecure
	}

	logic := bridge.NewBridgeLogic(0) // InactivityTimeout not needed for merging
	cfg := logic.MergeConfigs(base, fileCfg, credCfg, cliFlags)

	if err := logic.ValidateConfig(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func main() {
	// Detect if running under systemd (journald handles timestamps)
	if os.Getenv("JOURNAL_STREAM") != "" {
		log.SetFlags(0)
	}

	// Flags definition
	configPath := flag.String("config", "", "Path to configuration file (JSON)")
	credfilePath := flag.String("credfile", "", "Path to credentials file (username and password on separate lines)")
	device := flag.String("device", "", "Serial device path")
	broker := flag.String("broker", "", "MQTT broker address")
	topic := flag.String("topic", "", "MQTT topic")
	user := flag.String("user", "", "MQTT username")
	pass := flag.String("pass", "", "MQTT password")
	clientID := flag.String("clientid", "", "MQTT client ID (default: kamstrup2mqtt-<hostname>-<pid>)")
	qos := flag.Int("qos", -1, "MQTT QoS level (0, 1, or 2)")
	tlsCa := flag.String("tls-ca", "", "Path to CA certificate file for TLS")
	tlsCert := flag.String("tls-cert", "", "Path to client certificate file for mutual TLS")
	tlsKey := flag.String("tls-key", "", "Path to client private key file for mutual TLS")
	tlsInsecure := flag.Int("tls-insecure", -1, "Skip TLS certificate verification (0=no, 1=yes)")
	flag.Parse()

	// 1. Build configuration
	cfg, err := buildConfig(*configPath, *credfilePath, *device, *broker, *topic, *user, *pass, *clientID, *qos, *tlsCa, *tlsCert, *tlsKey, *tlsInsecure)
	if err != nil {
		log.Fatalf("Fatal error building configuration: %v", err)
	}

	// 2. Initialize MQTT
	client, err := bridge.InitMQTT(cfg)
	if err != nil {
		log.Fatalf("Fatal error connecting to MQTT: %v", err)
	}
	defer client.Disconnect(250)

	// 3. Setup context and signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 4. Start Bridge
	b := bridge.NewBridge(cfg, client)
	b.Start(ctx, cancel, sigChan)
}
