package main

import (
	"fmt"
	"time"
)

// BridgeLogic contains the pure business logic for the bridge.
// It is intended to be side-effect free (no I/O, no logging).
type BridgeLogic struct {
	InactivityTimeout time.Duration
}

// NewBridgeLogic creates a new BridgeLogic instance.
func NewBridgeLogic(inactivityTimeout time.Duration) *BridgeLogic {
	return &BridgeLogic{
		InactivityTimeout: inactivityTimeout,
	}
}

// CalculateTickerInterval determines the appropriate ticker interval based on watchdog settings.
func (l *BridgeLogic) CalculateTickerInterval(watchdogEnabled bool, watchdogInterval time.Duration, defaultInterval time.Duration) time.Duration {
	tickerInterval := defaultInterval
	if watchdogEnabled && watchdogInterval/2 > 0 && watchdogInterval/2 < tickerInterval {
		tickerInterval = watchdogInterval / 2
	}
	return tickerInterval
}

// ShouldWarnInactivity checks if an inactivity warning should be issued.
// It returns true if a warning is needed, and the updated lastWarning time.
func (l *BridgeLogic) ShouldWarnInactivity(lastActivity, lastWarning, now time.Time) (bool, time.Time) {
	if now.Sub(lastActivity) > l.InactivityTimeout {
		if now.Sub(lastWarning) > l.InactivityTimeout {
			return true, now
		}
	}
	return false, lastWarning
}

// CalculateBackoff computes the next backoff duration.
func (l *BridgeLogic) CalculateBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// ShouldLogSerialOpen determines if an "Opening serial port" message should be logged.
func (l *BridgeLogic) ShouldLogSerialOpen(firstAttempt bool, currentBackoff time.Duration) bool {
	return firstAttempt || currentBackoff > 1*time.Second
}

// MergeConfigs applies overrides to a base configuration in order.
// Later configs take precedence over earlier ones.
// For QoS, -1 is treated as "not set" and won't override.
func (l *BridgeLogic) MergeConfigs(base Config, overrides ...Config) Config {
	cfg := base

	for _, override := range overrides {
		if override.Device != "" {
			cfg.Device = override.Device
		}
		if override.Broker != "" {
			cfg.Broker = override.Broker
		}
		if override.Topic != "" {
			cfg.Topic = override.Topic
		}
		if override.User != "" {
			cfg.User = override.User
		}
		if override.Password != "" {
			cfg.Password = override.Password
		}
		if override.ClientID != "" {
			cfg.ClientID = override.ClientID
		}
		if override.QoS != nil {
			cfg.QoS = override.QoS
		}
		if override.TLSCa != "" {
			cfg.TLSCa = override.TLSCa
		}
		if override.TLSCert != "" {
			cfg.TLSCert = override.TLSCert
		}
		if override.TLSKey != "" {
			cfg.TLSKey = override.TLSKey
		}
		if override.TLSInsecure != nil {
			cfg.TLSInsecure = override.TLSInsecure
		}
	}

	return cfg
}

// ValidateConfig checks if the configuration is valid.
func (l *BridgeLogic) ValidateConfig(cfg Config) error {
	if cfg.QoS == nil {
		return fmt.Errorf("QoS level not set")
	}
	if *cfg.QoS < 0 || *cfg.QoS > 2 {
		return fmt.Errorf("invalid QoS level %d (must be 0, 1, or 2)", *cfg.QoS)
	}
	// TLS client cert and key must be specified together
	if (cfg.TLSCert != "") != (cfg.TLSKey != "") {
		return fmt.Errorf("tls_cert and tls_key must both be specified for client certificate authentication")
	}
	return nil
}
