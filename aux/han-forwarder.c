/*
  Kamstrup HAN → USB Serial Forwarder
  ----------------------------------
  This sketch turns an Arduino Nano into a transparent serial bridge
  for the Kamstrup power meter HAN port.

  - The Kamstrup HAN port outputs an inverted, open-collector UART signal
    at 2400 baud.
  - The Arduino reads this signal using SoftwareSerial with RX inversion.
  - All received bytes are forwarded unmodified to the USB serial port.

  No parsing, buffering, or protocol logic is applied. The raw data
  stream can be decoded on the host computer (e.g. Linux, Home Assistant,
  MQTT bridge, etc.).
*/

#include <SoftwareSerial.h>

/*
  Kamstrup HAN wiring:
  - RX pin connects to the HAN data line
  - TX pin is unused (HAN is transmit-only)
*/
#define HAN_RX_PIN 2
#define HAN_TX_PIN 3   // Dummy pin, not connected

/*
  SoftwareSerial instance:
  - Uses RX on pin 2
  - TX is unused
  - 'true' enables inverted logic, required by Kamstrup HAN
*/
SoftwareSerial inputData(HAN_RX_PIN, HAN_TX_PIN, true);

void setup() {
  /*
    The HAN port is open-collector, so the RX line must be pulled high.
    The signal is inverted in SoftwareSerial.
  */
  pinMode(HAN_RX_PIN, INPUT_PULLUP);

  /*
    USB serial output.
    Baud rate here is independent of the HAN port and can be higher
    if desired (e.g. 115200), but 2400 keeps it symmetric and simple.
  */
  Serial.begin(2400);

  /*
    Initialize the HAN input serial.
    Kamstrup HAN ports typically use 2400 baud, 8N1.
  */
  inputData.begin(2400);
}

void loop() {
  /*
    Forward all available bytes from the HAN port to USB serial.
    Data is passed through unchanged.
  */
  while (inputData.available()) {
    Serial.write(inputData.read());
  }
}

