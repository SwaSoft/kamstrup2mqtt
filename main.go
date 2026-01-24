package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

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
	client, err := initMQTT(cfg)
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
	bridge := NewBridge(cfg, client)
	bridge.Start(ctx, cancel, sigChan)
}
