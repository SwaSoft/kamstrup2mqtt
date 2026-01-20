# Serial-to-MQTT Bridge (`serial2mqtt`)

A robust serial-to-MQTT bridge implemented in Go, designed to read length-encoded binary frames from a serial port and publish them as binary payloads to an MQTT broker.

## Features

- **Framed Protocol Parsing**: Reconstructs frames starting and ending with `~` (0x7E), using a 12-bit length field.
- **Sanity Validation**: Enforces packet size limits (10 to 4097 bytes) to prevent malformed data processing.
- **Robust Serial I/O**: Single-byte read granularity with a 15-second timeout and automatic recovery from partial frames.
- **Persistent MQTT Connection**: Automatic reconnection with exponential back-off (up to 300s).
- **Inactivity Monitoring**: Emits a warning if no valid frames are published for 60 seconds.
- **Multi-Architecture Support**: Pre-compiled for AMD64 and ARM64.

## Installation

Download the appropriate binary for your platform:
- `serial2mqtt-amd64` (for AMD/Intel x86_64)
- `serial2mqtt-arm64` (for ARM 64-bit)

## Usage

```bash
./serial2mqtt [options]
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to configuration file (JSON) | (empty) |
| `-device` | Serial device path | `/dev/ttyUSB0` |
| `-baud` | Serial baud rate | `2400` |
| `-broker` | MQTT broker address | `tcp://localhost:1883` |
| `-topic` | MQTT topic | `serial/bridge` |
| `-user` | MQTT username | (empty) |
| `-pass` | MQTT password | (empty) |
| `-passfile` | Path to file containing MQTT password (overrides `-pass`) | (empty) |

> [!NOTE]
> Command line flags override values found in the configuration file.

### Example

1. Create a configuration file (`config.json`):
   ```json
   {
     "device": "/dev/ttyUSB1",
     "baud": 2400,
     "broker": "tcp://192.168.1.50:1883",
     "user": "my_user",
     "password": "my_password"
   }
   ```

2. Run the bridge:
   ```bash
   ./serial2mqtt -config config.json
   ```

3. Run with override:
   ```bash
   ./serial2mqtt -config config.json -device /dev/ttyUSB0
   ```

## Protocol Definition

Frames follow this structure:
`[ 0x7E ] [ MSB_LENGTH_NIBBLE ] [ LSB_LENGTH ] [ ...PAYLOAD... ] [ 0x7E ]`

- `0x7E` is the delimiter (`~`).
- `length = ((Byte1 & 0x0F) << 8) | Byte2`.
- `total_packet_size = length + 2`.

## Building from Source

Requires Go 1.22+.

```bash
go mod tidy
go build -o serial2mqtt
```

### Multi-Platform Builds

To compile for specific architectures (e.g., AMD64 or ARM64):

**AMD64 (x86_64):**
```bash
GOOS=linux GOARCH=amd64 go build -o serial2mqtt-amd64
```

**ARM64:**
```bash
GOOS=linux GOARCH=arm64 go build -o serial2mqtt-arm64
```
