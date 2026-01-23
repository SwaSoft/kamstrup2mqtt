# Kamstrup HAN Forwarder - Arduino Sketch

This Arduino sketch turns an Arduino Nano (or compatible board) into a transparent serial bridge for reading data from a Kamstrup power meter's HAN port.

## Hardware Requirements

- Arduino Nano (or Uno, Pro Mini, etc.)
- RJ12 cable or connector
- Jumper wires

## RJ12 Wiring (Kamstrup HAN Port)

The Kamstrup meter HAN port uses an RJ12 (6P6C) connector. Looking at the connector with the clip facing down and pins facing you:

```
  RJ12 Connector (front view)
  ┌─────────────────┐
  │ 1 2 3 4 5 6     │
  │ █ █ █ █ █ █     │
  └───────┬─────────┘
          │ clip
```

| Pin | Function     | Connect to Arduino |
|-----|--------------|-------------------|
| 1   | VCC (+5V)    | Not used          |
| 2   | Data Request | Not used          |
| 3   | Data Out     | Pin 2 (RX)        |
| 4   | Reserved     | Not used          |
| 5   | Reserved     | Not used          |
| 6   | GND          | GND               |

### Wiring Summary

Connect only two wires from the RJ12 cable to the Arduino:

```
Kamstrup HAN          Arduino Nano
─────────────         ────────────
Pin 3 (Data) ───────► Pin 2 (D2)
Pin 6 (GND)  ───────► GND
```

**Note:** The HAN data line is open-collector with inverted logic. The sketch enables the internal pull-up resistor on pin 2 and configures SoftwareSerial for inverted input.

## Installing the Arduino IDE

1. Download the Arduino IDE from https://www.arduino.cc/en/software
2. Install and launch the IDE

## Uploading the Sketch

1. Connect the Arduino to your computer via USB

2. Open the Arduino IDE

3. Open the sketch file `han-forwarder.c`:
   - File → Open → select `han-forwarder.c`
   - Or rename it to `han-forwarder.ino` first

4. Select your board:
   - Tools → Board → Arduino AVR Boards → Arduino Nano
   - For clone boards, set: Tools → Processor → ATmega328P (Old Bootloader)

5. Select the serial port:
   - Tools → Port → select the port (e.g., `/dev/ttyUSB0` on Linux, `COM3` on Windows)

6. Upload:
   - Click the Upload button (→) or press Ctrl+U
   - Wait for "Done uploading"

## Verifying Operation

1. Open the Serial Monitor (Tools → Serial Monitor)
2. Set baud rate to **2400**
3. You should see raw binary data appearing when the meter transmits

Alternatively, use a terminal:
```bash
# Linux
stty -F /dev/ttyUSB0 2400 raw
cat /dev/ttyUSB0 | xxd

# Or with screen
screen /dev/ttyUSB0 2400
```

## Using with kamstrup2mqtt

Once the Arduino is connected and forwarding data, configure kamstrup2mqtt to read from the Arduino's serial port:

```bash
./kamstrup2mqtt -serial /dev/ttyUSB0 -baud 2400 -mqtt tcp://localhost:1883
```

## Troubleshooting

- **No data received**: Check the RJ12 wiring, especially GND connection
- **Garbage data**: Verify baud rate is set to 2400
- **Upload fails on Nano clone**: Try "ATmega328P (Old Bootloader)" under Tools → Processor
- **Permission denied on Linux**: Add your user to the `dialout` group:
  ```bash
  sudo usermod -aG dialout $USER
  ```
  Then log out and back in.
