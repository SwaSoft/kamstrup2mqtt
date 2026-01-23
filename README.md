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

### Interactive Configuration

During installation, you'll be prompted for:

| Setting | Default |
|---------|---------|
| Serial device path | `/dev/ttyUSB0` |
| MQTT broker URI | `tcp://localhost:1883` |
| MQTT topic | `serial/bridge` |
| MQTT client ID | `kamstrup2mqtt-system` |
| QoS level | `0` |
| MQTT username | (none) |
| MQTT password | (none) |

The installer then:
- Creates a `kamstrup2mqtt` system user with `dialout` group membership
- Writes `/etc/kamstrup2mqtt/config.json` from your answers
- If credentials were provided, stores them securely in `/etc/kamstrup2mqtt/mqtt.credentials`
- Enables and starts the systemd service

### Non-Interactive Installation

To install without prompts (e.g., for automation), preseed debconf:

```bash
sudo debconf-set-selections <<EOF
kamstrup2mqtt kamstrup2mqtt/device string /dev/ttyUSB0
kamstrup2mqtt kamstrup2mqtt/broker string tcp://192.168.1.100:1883
kamstrup2mqtt kamstrup2mqtt/topic string serial/bridge
kamstrup2mqtt kamstrup2mqtt/client_id string kamstrup2mqtt
kamstrup2mqtt kamstrup2mqtt/qos select 0
kamstrup2mqtt kamstrup2mqtt/mqtt_user string myuser
kamstrup2mqtt kamstrup2mqtt/mqtt_password password mypassword
EOF
sudo DEBIAN_FRONTEND=noninteractive dpkg -i kamstrup2mqtt_*.deb
```

### Reconfiguring

To change settings after installation:

```bash
sudo dpkg-reconfigure kamstrup2mqtt
```

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

If not using the `.deb` package, download the binary for your architecture from the releases page and install manually:

1. Copy binary:
   ```bash
   sudo cp kamstrup2mqtt-amd64 /usr/bin/kamstrup2mqtt
   sudo chmod +x /usr/bin/kamstrup2mqtt
   ```

2. Create service user:
   ```bash
   sudo useradd -r -s /usr/sbin/nologin -G dialout kamstrup2mqtt
   ```

3. Create config directory and file:
   ```bash
   sudo mkdir -p /etc/kamstrup2mqtt
   sudo tee /etc/kamstrup2mqtt/config.json <<EOF
   {
     "device": "/dev/ttyUSB0",
     "broker": "tcp://localhost:1883",
     "topic": "serial/bridge",
     "client_id": "kamstrup2mqtt",
     "qos": 0
   }
   EOF
   sudo chown kamstrup2mqtt:kamstrup2mqtt /etc/kamstrup2mqtt/config.json
   sudo chmod 640 /etc/kamstrup2mqtt/config.json
   ```

4. Create systemd unit file:
   ```bash
   sudo tee /etc/systemd/system/kamstrup2mqtt.service <<EOF
   [Unit]
   Description=Kamstrup to MQTT Bridge
   After=network-online.target
   Wants=network-online.target

   [Service]
   Type=notify
   ExecStart=/usr/bin/kamstrup2mqtt -config /etc/kamstrup2mqtt/config.json
   Restart=on-failure
   RestartSec=5
   WatchdogSec=30
   User=kamstrup2mqtt
   Group=dialout
   NoNewPrivileges=yes
   ProtectSystem=strict
   ProtectHome=yes
   PrivateTmp=yes
   ReadOnlyPaths=/
   ReadWritePaths=/dev

   [Install]
   WantedBy=multi-user.target
   EOF
   ```

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

## Home Assistant Integration

This program has been tested and verified to work with the [AMS HAN Meter Reader](https://github.com/toreamun/amshan-homeassistant) integration for Home Assistant. Configure the integration to subscribe to the MQTT topic used by kamstrup2mqtt.
