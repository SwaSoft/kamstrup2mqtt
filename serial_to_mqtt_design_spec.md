# Serial-to-MQTT Bridge – Design Specification

## 1. Purpose and High-Level Behavior

The program acts as a **serial-to-MQTT bridge** for framed binary data.

It continuously:
1. Reads raw bytes from a serial port.
2. Detects and reconstructs framed packets using a simple length-encoded protocol.
3. Validates packet structure and size.
4. Publishes valid frames as binary payloads to an MQTT topic.
5. Logs operational status and warnings to standard output and standard error.

The program runs indefinitely until interrupted.

---

## 2. External Dependencies and Interfaces

### 2.1 Serial Interface

**Role**: Input source of binary frames.

**Configuration**:
- Device path: configurable string (e.g. `/dev/ttyUSB0`)
- Baud rate: `2400`
- Data bits: `8`
- Parity: none
- Stop bits: `1`
- Flow control: disabled (RTS/CTS, DSR/DTR, XON/XOFF all off)
- Read timeout: `15 seconds`

**Read granularity**:
- Exactly **one byte at a time**

**Behavior on timeout**:
- A read returns “no data” after timeout expiration.

---

### 2.2 MQTT Interface

**Role**: Output channel for valid frames.

**Configuration**:
- Broker host (IP or hostname)
- Topic name
- Username
- Password read from a file

**Behavior**:
- Establish a single persistent connection.
- Frames that fails to be published to MQTT can be discarded.
- Run the MQTT network loop asynchronously (background thread or event loop).
- Publish each complete frame as a **binary payload** (no transformation).
- No QoS, retain flag, or response handling is required.

---

### 2.3 File System

Used only to read the MQTT password.

**Requirements**:
- File must exist.
- File must be readable.
- Entire contents are read and trimmed of whitespace.

**Failure handling**:
- Missing or unreadable file is a fatal error.

---

## 3. Frame Protocol Definition

### 3.1 Frame Structure

Each frame is delimited and length-encoded:

```
+------------+------------+------------+------------------+------------+
| Byte 0     | Byte 1     | Byte 2     | Payload bytes    | Last byte  |
+------------+------------+------------+------------------+------------+
| '~' (0x7E) | Length MSB | Length LSB | Variable length  | '~' (0x7E) |
+------------+------------+------------+------------------+------------+
```

---

### 3.2 Length Field Interpretation

- The **length field** is formed from **Byte 1 and Byte 2**
- Only the **low nibble (4 bits)** of Byte 1 is used
- Computation:

```
length = ((Byte1 & 0x0F) << 8) | Byte2
```

- The computed value represents the frame size **excluding**:
  - The starting delimiter (`~`)
  - The ending delimiter (`~`)

---

### 3.3 Total Packet Size

```
total_packet_size = length + 2
```

This includes:
- 1 byte start delimiter
- Payload and metadata
- 1 byte end delimiter

---

### 3.4 Sanity Bounds

To prevent malformed or runaway frames:

- Minimum packet size: `10 bytes`
- Maximum packet size: `2048 bytes`

If the computed total size is outside this range, the frame is discarded immediately.

---

## 4. Internal State Machine

The serial parsing logic behaves as a **byte-driven state machine**.

### 4.1 State Variables

| Variable | Purpose |
|--------|--------|
| `frame_started` | Whether a frame start delimiter has been seen |
| `frame_buffer` | Accumulated bytes of the current frame |
| `byte_counter` | Number of bytes collected so far |
| `expected_packet_size` | Total size of the current frame |
| `last_activity_timestamp` | Time of last successfully published frame |

---

### 4.2 State Transitions

#### Idle State
- `frame_started = false`
- Bytes are ignored **until** a `~` byte is received.

#### Start of Frame
- On receiving `~` while idle:
  - Reset all frame state
  - Start collecting bytes
  - `frame_started = true`

#### Frame Accumulation
- Each subsequent byte is appended to the frame buffer.
- Increment byte counter.

#### Length Determination
- When exactly 3 bytes have been collected:
  - Compute expected packet size.
  - If outside sanity bounds:
    - Discard frame
    - Return to Idle state

#### Frame Completion
- When `byte_counter == expected_packet_size`:
  - Validate that the **last byte is `~`**
    - If valid:
      - Publish frame
      - Update activity timestamp
    - If invalid:
      - Discard frame
  - Reset state to Idle

---

## 5. Timeout and Error Handling

### 5.1 Serial Read Timeout

If a serial read times out (no data for `timeout` seconds):

- If currently inside a frame:
  - Discard the partial frame
  - Log a warning
  - Reset to Idle

---

### 5.2 Inactivity Warning

If **no valid frame has been published for more than 60 seconds**:

- Emit a warning to standard error
- Reset the inactivity timer so the warning repeats at most once per minute

---

### 5.3 MQTT Errors

- Failure to connect to broker is fatal.
  - Reconnect automatically if needed, log if successful.
  - Unlimited retries with exponential back-off to a maximum of 300 seconds.
- Publish errors are logged but do not stop execution.

---

### 5.4 Graceful Shutdown

- On user interrupt (e.g. Ctrl-C):
  - Stop MQTT network loop
  - Disconnect MQTT client
  - Exit cleanly

---

## 6. Logging and Observability

### 6.1 Standard Output

- Startup messages
- Successful frame publication messages:
  - Timestamp
  - Frame length

### 6.2 Standard Error

- Password file errors
- MQTT connection failures
- Invalid frame size
- Malformed frames
- Timeout discards
- Inactivity warnings

Logging is human-readable and line-oriented.

---

## 7. Concurrency Model

- Serial reading is **blocking** and sequential.
- MQTT network handling runs concurrently (thread or async loop).
- No shared mutable state beyond timestamps and counters.
- No synchronization primitives required beyond basic variable access.

---

## 8. Termination Conditions

The program runs indefinitely unless:
- A fatal initialization error occurs, or
- The user interrupts execution.

There is no automatic restart or watchdog logic.

---

## 9. Portability Notes

To re-implement in another language, the following capabilities are required:

- Byte-level serial I/O with configurable timeout
- MQTT client supporting:
  - Username/password authentication
  - Binary payload publishing
  - Background network processing
- Access to system clock (monotonic or wall clock)
- File I/O with error detection
- Graceful signal handling

No Python-specific features are relied upon.

---

## 10. Summary

This program is a **robust framed-protocol reader** that:
- Enforces strict framing and length validation
- Protects against partial, malformed, or stalled data
- Acts as a transparent transport bridge from serial to MQTT
- Prioritizes reliability over throughput
