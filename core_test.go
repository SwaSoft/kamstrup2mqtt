package main

import (
	"testing"
	"time"
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
