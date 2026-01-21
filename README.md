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
| `-clientid` | MQTT client ID | `serial2mqtt-<hostname>-<pid>` |
| `-qos` | MQTT QoS level (0, 1, or 2) | `0` |

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

3. (Optional) Create a password file with restricted permissions:
   ```bash
   echo "your_mqtt_password" | sudo tee /etc/serial2mqtt/mqtt.pass
   sudo chmod 600 /etc/serial2mqtt/mqtt.pass
   ```

4. Add your user to the `dialout` group for serial port access (or run as root):
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
ExecStart=/usr/local/bin/serial2mqtt -config /etc/serial2mqtt/config.json -passfile /etc/serial2mqtt/mqtt.pass
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
