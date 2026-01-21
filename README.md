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
| `-credfile` | Path to credentials file (username and password on separate lines) | (empty) |
| `-device` | Serial device path | `/dev/ttyUSB0` |
| `-baud` | Serial baud rate | `2400` |
| `-broker` | MQTT broker address | `tcp://localhost:1883` |
| `-topic` | MQTT topic | `serial/bridge` |
| `-user` | MQTT username | (empty) |
| `-pass` | MQTT password | (empty) |
| `-clientid` | MQTT client ID | `serial2mqtt-<hostname>-<pid>` |
| `-qos` | MQTT QoS level (0, 1, or 2) | `0` |

### Configuration Priority

Configuration values are applied in the following order (highest priority last):

1. Defaults
2. JSON config file (`-config`)
3. Credentials file (`-credfile`)
4. Command-line flags (`-user`, `-pass`, etc.)

This means CLI flags always win, credentials file overrides the JSON config, and the JSON config overrides defaults.

### Example

1. Create a configuration file (`config.json`):
   ```json
   {
     "device": "/dev/ttyUSB1",
     "baud": 2400,
     "broker": "tcp://192.168.1.50:1883",
     "user": "my_user",
     "password": "my_password",
     "client_id": "my-bridge",
     "qos": 1
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

### Using Credentials File

For secure credential management (especially with systemd), you can use a separate credentials file:

1. Create a credentials file with username on the first line and password on the second:
   ```
   my_mqtt_user
   my_mqtt_password
   ```

2. Run the bridge with the credentials file:
   ```bash
   ./serial2mqtt -config config.json -credfile /path/to/credentials
   ```

The credentials file format supports:
- Passwords with special characters (including colons)
- Leading/trailing whitespace is trimmed from both lines

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

## Testing

Run all tests:
```bash
go test ./...
```

Run a specific test:
```bash
go test -run TestFrameParser_ValidFrame ./...
```

Run tests with verbose output:
```bash
go test -v ./...
```

## Running as a systemd Service

The application includes native systemd integration with readiness notification and watchdog support.

### Installation

1. Copy the binary to a system location:
   ```bash
   sudo cp serial2mqtt /usr/local/bin/
   sudo chmod +x /usr/local/bin/serial2mqtt
   ```

2. Create a configuration file:
   ```bash
   sudo mkdir -p /etc/serial2mqtt
   sudo nano /etc/serial2mqtt/config.json
   ```

3. Add your user to the `dialout` group for serial port access (or run as root):
   ```bash
   sudo usermod -aG dialout serial2mqtt
   ```

### Unit File

Create `/etc/systemd/system/serial2mqtt.service`:

```ini
[Unit]
Description=Serial to MQTT Bridge
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
ExecStart=/usr/local/bin/serial2mqtt -config /etc/serial2mqtt/config.json
Restart=on-failure
RestartSec=5

# Watchdog: service must send keep-alive within this interval
WatchdogSec=30

# Security hardening
User=serial2mqtt
Group=dialout
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadOnlyPaths=/
ReadWritePaths=/dev

[Install]
WantedBy=multi-user.target
```

### Secure Credentials with LoadCredential

For enhanced security, use systemd's `LoadCredential` directive to manage MQTT credentials separately from the main configuration:

1. Create a credentials file:
   ```bash
   sudo mkdir -p /etc/serial2mqtt
   sudo nano /etc/serial2mqtt/mqtt.credentials
   # Add username on first line, password on second line
   sudo chmod 600 /etc/serial2mqtt/mqtt.credentials
   ```

2. Update the unit file to use `LoadCredential`:
   ```ini
   [Service]
   Type=notify
   LoadCredential=mqtt-creds:/etc/serial2mqtt/mqtt.credentials
   ExecStart=/usr/local/bin/serial2mqtt \
       -config /etc/serial2mqtt/config.json \
       -credfile ${CREDENTIALS_DIRECTORY}/mqtt-creds
   ```

This approach:
- Keeps credentials separate from the JSON config file
- Allows credentials to be stored with restricted permissions (600)
- Credentials are only accessible to the service process via `CREDENTIALS_DIRECTORY`
- Works seamlessly with secret management tools that populate credential files

### Create Service User

```bash
sudo useradd -r -s /usr/sbin/nologin -G dialout serial2mqtt
```

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable serial2mqtt
sudo systemctl start serial2mqtt
```

### Monitoring

```bash
# View logs
journalctl -u serial2mqtt -f

# Check status
systemctl status serial2mqtt
```

### Watchdog Behavior

When `WatchdogSec` is configured, the application automatically sends keep-alive notifications to systemd. If the application becomes unresponsive (e.g., deadlock), systemd will restart it after the watchdog timeout expires.
