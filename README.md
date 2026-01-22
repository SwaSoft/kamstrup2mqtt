# kamstrup2mqtt

A serial-to-MQTT bridge for Kamstrup power meters. Reads length-encoded binary frames from the meter's serial stream and publishes them to an MQTT broker.

## Features

- Parses framed protocol with `0x7E` delimiters and 12-bit length field
- Automatic reconnection with exponential back-off for both serial and MQTT
- Inactivity monitoring with warnings after 60 seconds of no valid frames
- systemd integration with watchdog support
- Pre-built packages for AMD64 and ARM64

## Installation

Download the `.deb` package for your platform from the releases page:

| Architecture | Package |
|--------------|---------|
| AMD/Intel x86_64 | `kamstrup2mqtt_*_amd64.deb` |
| ARM 64-bit | `kamstrup2mqtt_*_arm64.deb` |

```bash
sudo dpkg -i kamstrup2mqtt_*.deb
```

The package creates a `kamstrup2mqtt` system user, installs the binary to `/usr/bin/`, sets up a default configuration at `/etc/kamstrup2mqtt/config.json`, and enables the systemd service.

## Configuration

Edit `/etc/kamstrup2mqtt/config.json`:

```json
{
  "device": "/dev/ttyUSB0",
  "broker": "tcp://localhost:1883",
  "topic": "serial/bridge",
  "user": "",
  "password": "",
  "client_id": "kamstrup2mqtt",
  "qos": 0
}
```

Restart the service after making changes:

```bash
sudo systemctl restart kamstrup2mqtt
```

### Command-Line Options

All settings can also be specified via flags, which override the config file:

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to JSON config file | (none) |
| `-credfile` | Path to credentials file | (none) |
| `-device` | Serial device path | `/dev/ttyUSB0` |
| `-broker` | MQTT broker address | `tcp://localhost:1883` |
| `-topic` | MQTT topic | `serial/bridge` |
| `-user` | MQTT username | (none) |
| `-pass` | MQTT password | (none) |
| `-clientid` | MQTT client ID | `kamstrup2mqtt-<hostname>-<pid>` |
| `-qos` | MQTT QoS level (0, 1, or 2) | `0` |

### Credentials File

For secure credential management, store MQTT credentials in a separate file with the username on the first line and password on the second:

```
my_mqtt_user
my_mqtt_password
```

```bash
./kamstrup2mqtt -config config.json -credfile /path/to/credentials
```

### Priority

Configuration is applied in order (later sources override earlier):

1. Defaults
2. JSON config file (`-config`)
3. Credentials file (`-credfile`)
4. Command-line flags

## Service Management

```bash
# Check status
systemctl status kamstrup2mqtt

# View logs
journalctl -u kamstrup2mqtt -f

# Restart
sudo systemctl restart kamstrup2mqtt
```

When `WatchdogSec` is configured in the unit file, the application sends periodic keep-alive notifications to systemd. If the application becomes unresponsive, systemd will restart it.

## Building from Source

Requires Go 1.22+.

```bash
go build -o kamstrup2mqtt
```

### Cross-Compilation

```bash
GOOS=linux GOARCH=amd64 go build -o kamstrup2mqtt-amd64
GOOS=linux GOARCH=arm64 go build -o kamstrup2mqtt-arm64
```

### Debian Packages

Build `.deb` packages for both architectures:

```bash
make deb
```

Creates `dist/kamstrup2mqtt_<version>_amd64.deb` and `dist/kamstrup2mqtt_<version>_arm64.deb`. The version is derived from git tags or the commit hash.

### Manual Installation

If not using the `.deb` package:

1. Copy binary:
   ```bash
   sudo cp kamstrup2mqtt /usr/local/bin/
   ```

2. Create config directory and file:
   ```bash
   sudo mkdir -p /etc/kamstrup2mqtt
   sudo nano /etc/kamstrup2mqtt/config.json
   ```

3. Create service user:
   ```bash
   sudo useradd -r -s /usr/sbin/nologin -G dialout kamstrup2mqtt
   ```

4. Create a systemd unit file at `/etc/systemd/system/kamstrup2mqtt.service`

5. Enable and start:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now kamstrup2mqtt
   ```

## Testing

```bash
go test ./...
```

## Protocol

Frames use this structure:

```
[ 0x7E ] [ LENGTH_HIGH ] [ LENGTH_LOW ] [ PAYLOAD... ] [ 0x7E ]
```

- `0x7E` is the frame delimiter
- Length is 12 bits: `((byte1 & 0x0F) << 8) | byte2`
- Total packet size = length + 2 (for delimiters)
- Valid packet size: 10-4097 bytes
