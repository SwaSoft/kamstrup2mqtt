# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Build Commands

```bash
# Build
go build -o kamstrup2mqtt

# Run tests
go test ./...

# Run a single test
go test -run TestFrameParser_ValidFrame ./...

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o kamstrup2mqtt-amd64
GOOS=linux GOARCH=arm64 go build -o kamstrup2mqtt-arm64
```

## Architecture

This is a single-file Go application (`main.go`) that bridges serial port data to MQTT. The codebase is intentionally minimal.

### Core Components

**FrameParser** - State machine that reconstructs length-encoded binary frames from a Kamstrup power meter's serial stream. Frames are delimited by `0x7E` with a 12-bit length field in bytes 1-2. The parser handles resynchronization when unexpected delimiters appear mid-frame.

**Main loop** - Runs serial reading in a goroutine with exponential backoff reconnection. Uses `context.Context` for coordinated shutdown. A separate ticker monitors for inactivity (no valid frames for 60 seconds) and sends systemd watchdog notifications when enabled.

### Frame Protocol

```
[ 0x7E ] [ MSB_LENGTH_NIBBLE ] [ LSB_LENGTH ] [ ...PAYLOAD... ] [ 0x7E ]
```

- Length = `((Byte1 & 0x0F) << 8) | Byte2`
- Total packet size = length + 2 (for delimiters)
- Valid range: 10-4097 bytes

### Configuration Priority

1. Defaults (in `DefaultConfig()`)
2. JSON config file (`-config`)
3. Command-line flags (highest priority)

### Dependencies

- `github.com/eclipse/paho.mqtt.golang` - MQTT client
- `github.com/tarm/serial` - Serial port handling
- `github.com/coreos/go-systemd/v22/daemon` - systemd notify/watchdog support

### Systemd Integration

When running under systemd with `Type=notify`, the application:
- Sends `READY=1` after MQTT connection is established
- Sends `STOPPING=1` on shutdown
- Sends `WATCHDOG=1` periodically when `WatchdogSec` is configured
- Automatically disables log timestamps (journald provides them)
