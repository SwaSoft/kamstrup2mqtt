package main

import (
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
