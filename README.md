# Kamstrup-to-MQTT Bridge (`kamstrup2mqtt`)

A robust serial-to-MQTT bridge implemented in Go, designed to read length-encoded binary frames from a Kamstrup power meter's serial stream and publish them as binary payloads to an MQTT broker.

## Features

- **Framed Protocol Parsing**: Reconstructs frames starting and ending with `~` (0x7E), using a 12-bit length field.
- **Sanity Validation**: Enforces packet size limits (10 to 4097 bytes) to prevent malformed data processing.
- **Robust Serial I/O**: Single-byte read granularity with a 15-second timeout and automatic recovery from partial frames.
- **Persistent MQTT Connection**: Automatic reconnection with exponential back-off (up to 300s).
- **Inactivity Monitoring**: Emits a warning if no valid frames are published for 60 seconds.
- **Multi-Architecture Support**: Pre-compiled for AMD64 and ARM64.

Download the appropriate `.deb` package for your platform from the releases page:
- `kamstrup2mqtt_*_amd64.deb` (for AMD/Intel x86_64)
- `kamstrup2mqtt_*_arm64.deb` (for ARM 64-bit)

Install using `dpkg`:
```bash
sudo dpkg -i kamstrup2mqtt_*.deb
```

The package will automatically:
- Create the `kamstrup2mqtt` system user.
- Add the user to the `dialout` group.
- Install the binary to `/usr/bin/kamstrup2mqtt`.
- Set up a default configuration at `/etc/kamstrup2mqtt/config.json`.
- Install and enable the `systemd` service.

## Usage

```bash
./kamstrup2mqtt [options]
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
| `-clientid` | MQTT client ID | `kamstrup2mqtt-<hostname>-<pid>` |
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
   ./kamstrup2mqtt -config config.json
   ```

3. Run with override:
   ```bash
   ./kamstrup2mqtt -config config.json -device /dev/ttyUSB0
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
   ./kamstrup2mqtt -config config.json -credfile /path/to/credentials
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
go build -o kamstrup2mqtt
```

### Multi-Platform Builds

To compile for specific architectures (e.g., AMD64 or ARM64):

**AMD64 (x86_64):**
```bash
GOOS=linux GOARCH=amd64 go build -o kamstrup2mqtt-amd64
```

**ARM64:**
```bash
GOOS=linux GOARCH=arm64 go build -o kamstrup2mqtt-arm64
```

Or use the Makefile:
```bash
make build-cross
```

### Building Debian Packages

To build `.deb` packages for both amd64 and arm64:

**Prerequisites:**
- Go 1.22+
- `dpkg-deb` (part of the `dpkg` package, included on Debian/Ubuntu systems)

**Build the packages:**
```bash
make deb
```

This will:
1. Cross-compile binaries for both amd64 and arm64
2. Create `dist/kamstrup2mqtt_<version>_amd64.deb`
3. Create `dist/kamstrup2mqtt_<version>_arm64.deb`

The version is derived from git tags (e.g., `v1.0.0`) or the commit hash if no tags exist.

**Build everything (tarball + deb packages):**
```bash
make package
```

This additionally creates a `dist/kamstrup2mqtt-<version>.tar.gz` tarball containing both binaries and the manual installation script.

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

The easiest way to run as a service is by installing the `.deb` package, which handles all setup automatically.

### Manual Installation (from source/tarball)

If you are not using the `.deb` package, follow these steps:

1. Copy the binary to a system location:
   ```bash
   sudo cp kamstrup2mqtt /usr/local/bin/
   sudo chmod +x /usr/local/bin/kamstrup2mqtt
   ```

2. Create a configuration file:
   ```bash
   sudo mkdir -p /etc/kamstrup2mqtt
   sudo nano /etc/kamstrup2mqtt/config.json
   ```

3. Create the service user:
   ```bash
   sudo useradd -r -s /usr/sbin/nologin -G dialout kamstrup2mqtt
   ```

4. Create `/etc/systemd/system/kamstrup2mqtt.service` (see [Unit File](#unit-file) below).

5. Enable and Start:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable kamstrup2mqtt
   sudo systemctl start kamstrup2mqtt
   ```

### Monitoring

```bash
# View logs
journalctl -u kamstrup2mqtt -f

# Check status
systemctl status kamstrup2mqtt
```

### Watchdog Behavior

When `WatchdogSec` is configured, the application automatically sends keep-alive notifications to systemd. If the application becomes unresponsive (e.g., deadlock), systemd will restart it after the watchdog timeout expires.
