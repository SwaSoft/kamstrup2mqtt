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

// MergeConfigs applies overrides (cli then file) to a base configuration.
func (l *BridgeLogic) MergeConfigs(base, fileConfig, cliFlags Config) Config {
	cfg := base

	// Apply file overrides
	if fileConfig.Device != "" {
		cfg.Device = fileConfig.Device
	}
	if fileConfig.Baud != 0 {
		cfg.Baud = fileConfig.Baud
	}
	if fileConfig.Broker != "" {
		cfg.Broker = fileConfig.Broker
	}
	if fileConfig.Topic != "" {
		cfg.Topic = fileConfig.Topic
	}
	if fileConfig.User != "" {
		cfg.User = fileConfig.User
	}
	if fileConfig.Password != "" {
		cfg.Password = fileConfig.Password
	}
	if fileConfig.ClientID != "" {
		cfg.ClientID = fileConfig.ClientID
	}
	if fileConfig.QoS != 0 {
		cfg.QoS = fileConfig.QoS
	}

	// Apply CLI overrides
	if cliFlags.Device != "" {
		cfg.Device = cliFlags.Device
	}
	if cliFlags.Baud != 0 {
		cfg.Baud = cliFlags.Baud
	}
	if cliFlags.Broker != "" {
		cfg.Broker = cliFlags.Broker
	}
	if cliFlags.Topic != "" {
		cfg.Topic = cliFlags.Topic
	}
	if cliFlags.User != "" {
		cfg.User = cliFlags.User
	}
	if cliFlags.Password != "" {
		cfg.Password = cliFlags.Password
	}
	if cliFlags.ClientID != "" {
		cfg.ClientID = cliFlags.ClientID
	}
	if cliFlags.QoS != -1 { // -1 is our "not set" sentinel for QoS CLI flag
		cfg.QoS = cliFlags.QoS
	}

	return cfg
}

// ValidateConfig checks if the configuration is valid.
func (l *BridgeLogic) ValidateConfig(cfg Config) error {
	if cfg.QoS < 0 || cfg.QoS > 2 {
		return fmt.Errorf("invalid QoS level %d (must be 0, 1, or 2)", cfg.QoS)
	}
	return nil
}
