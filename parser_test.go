package main

import (
	"bytes"
	"os"
	"testing"
)

func TestFrameParser_ValidFrame(t *testing.T) {
	parser := &FrameParser{}
	// Payload length 8: 0x0008. Total size = 8 + 2 = 10.
	payload := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}

	var lastFrame []byte
	var lastErr error

	for _, b := range payload {
		frame, err := parser.ProcessByte(b)
		if err != nil {
			lastErr = err
		}
		if frame != nil {
			lastFrame = frame
		}
	}

	if lastErr != nil {
		t.Errorf("expected no error, got %v", lastErr)
	}
	if !bytes.Equal(lastFrame, payload) {
		t.Errorf("expected frame %v, got %v", payload, lastFrame)
	}
}

func TestFrameParser_InvalidSize(t *testing.T) {
	parser := &FrameParser{}
	// Length field 1: 0x0001. Total size = 1 + 2 = 3. Min is 10.
	payload := []byte{0x7E, 0x00, 0x01}

	var lastErr error
	for _, b := range payload {
		_, err := parser.ProcessByte(b)
		if err != nil {
			lastErr = err
		}
	}

	if lastErr == nil {
		t.Error("expected error for invalid size, got nil")
	}
}

func TestFrameParser_MalformedEnd(t *testing.T) {
	parser := &FrameParser{}
	// Payload length 8: 0x0008. Total size = 10. Last byte is 'X' not 0x7E.
	payload := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 'X'}

	var lastErr error
	for _, b := range payload {
		_, err := parser.ProcessByte(b)
		if err != nil {
			lastErr = err
		}
	}

	if lastErr == nil {
		t.Error("expected error for malformed end delimiter, got nil")
	}
}

func TestFrameParser_PartialAndReset(t *testing.T) {
	parser := &FrameParser{}
	// Start a frame but don't finish
	payload1 := []byte{0x7E, 0x00, 0x08, '1'}
	for _, b := range payload1 {
		parser.ProcessByte(b)
	}

	if !parser.frameStarted {
		t.Error("expected frameStarted to be true")
	}

	parser.Reset()

	if parser.frameStarted {
		t.Error("expected frameStarted to be false after reset")
	}

	// Should be able to parse a new valid frame after reset
	payload2 := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}
	var lastFrame []byte
	for _, b := range payload2 {
		frame, _ := parser.ProcessByte(b)
		if frame != nil {
			lastFrame = frame
		}
	}

	if !bytes.Equal(lastFrame, payload2) {
		t.Errorf("expected frame %v, got %v", payload2, lastFrame)
	}
}

func TestFrameParser_MaxLimit(t *testing.T) {
	parser := &FrameParser{}
	// Length field 4096: 0x1000. Total size = 4096 + 2 = 4098. Max is 4097.
	payload := []byte{0x7E, 0x10, 0x00}

	var lastErr error
	for _, b := range payload {
		_, err := parser.ProcessByte(b)
		if err != nil {
			lastErr = err
		}
	}

	if lastErr == nil {
		t.Error("expected error for exceeding max size, got nil")
	}
}

func TestFrameParser_LargeFrame(t *testing.T) {
	parser := &FrameParser{}
	// Large frame reported by user: size 3746 (length = 3744)
	length := 3744
	payload := make([]byte, length+2)
	payload[0] = 0x7E
	payload[1] = byte((length >> 8) & 0x0F)
	payload[2] = byte(length & 0xFF)
	for i := 3; i < length+1; i++ {
		payload[i] = 'A'
	}
	payload[length+1] = 0x7E

	var lastFrame []byte
	var lastErr error
	for _, b := range payload {
		frame, err := parser.ProcessByte(b)
		if err != nil {
			lastErr = err
		}
		if frame != nil {
			lastFrame = frame
		}
	}

	if lastErr != nil {
		t.Errorf("expected no error for large frame, got %v", lastErr)
	}
	if !bytes.Equal(lastFrame, payload) {
		t.Errorf("expected large frame to be parsed correctly")
	}
}

func TestFrameParser_SilentResync(t *testing.T) {
	parser := &FrameParser{}
	// Test: multiple delimiters at start should be handled silently
	payload := []byte{0x7E, 0x7E, 0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}

	var lastFrame []byte
	for _, b := range payload {
		frame, err := parser.ProcessByte(b)
		if err != nil {
			t.Errorf("expected no error for redundant delimiters, got %v", err)
		}
		if frame != nil {
			lastFrame = frame
		}
	}

	expected := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}
	if !bytes.Equal(lastFrame, expected) {
		t.Errorf("expected frame %v, got %v", expected, lastFrame)
	}
}

func TestFrameParser_DelimiterInPayload(t *testing.T) {
	parser := &FrameParser{}
	// Test: delimiter (0x7E) inside payload should be treated as data
	// payload length 8: 0x0008. Total size 10.
	// Byte 5 is 0x7E.
	payload := []byte{0x7E, 0x00, 0x08, '1', '2', 0x7E, '4', '5', '6', 0x7E}

	var lastFrame []byte
	var lastErr error
	for _, b := range payload {
		frame, err := parser.ProcessByte(b)
		if err != nil {
			lastErr = err
		}
		if frame != nil {
			lastFrame = frame
		}
	}

	if lastErr != nil {
		t.Errorf("expected no error for delimiter in payload, got %v", lastErr)
	}
	if !bytes.Equal(lastFrame, payload) {
		t.Errorf("expected frame %v, got %v", payload, lastFrame)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Device != "/dev/ttyUSB0" {
		t.Errorf("expected default device /dev/ttyUSB0, got %s", cfg.Device)
	}
	if cfg.Baud != 2400 {
		t.Errorf("expected default baud 2400, got %d", cfg.Baud)
	}
}

func TestConfigIO(t *testing.T) {
	// Test LoadConfig with non-existent file
	_, err := LoadConfig("non-existent-file.json")
	if err == nil {
		t.Error("expected error for non-existent config file")
	}

	// Test LoadConfig with valid JSON
	tmpFile, err := os.CreateTemp("", "config*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `{"device": "/dev/test", "baud": 9600}`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Errorf("failed to load config: %v", err)
	}
	if cfg.Device != "/dev/test" || cfg.Baud != 9600 {
		t.Errorf("loaded config mismatch: %+v", cfg)
	}
}

func TestReadPassword(t *testing.T) {
	// Write dummy password file
	tmpFile, err := os.CreateTemp("", "pass*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	password := "  secret123  \n"
	if _, err := tmpFile.Write([]byte(password)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	pass, err := readPassword(tmpFile.Name())
	if err != nil {
		t.Errorf("failed to read password: %v", err)
	}
	if pass != "secret123" {
		t.Errorf("expected 'secret123', got '%s'", pass)
	}

	// Test error on missing file
	_, err = readPassword("missing-password-file")
	if err == nil {
		t.Error("expected error for missing password file")
	}
}

func TestNewFrameParser(t *testing.T) {
	p := NewFrameParser()
	if cap(p.frameBuffer) != MaxPacketSize {
		t.Errorf("expected capacity %d, got %d", MaxPacketSize, cap(p.frameBuffer))
	}
}

func TestFrameParser_MinSize(t *testing.T) {
	parser := &FrameParser{}
	// MinPacketSize is 10. Payload length 8: 1+2+8+1 = 12 (OK)
	// Let's test length that results in exactly 10.
	payload := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}
	for _, b := range payload {
		frame, _ := parser.ProcessByte(b)
		if frame != nil {
			if len(frame) != 10 {
				t.Errorf("expected frame size 10, got %d", len(frame))
			}
		}
	}
}
func TestFrameParser_Stream(t *testing.T) {
	parser := &FrameParser{}
	// Two valid frames back to back
	frame1 := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5', '6', 0x7E}
	frame2 := []byte{0x7E, 0x00, 0x08, 'A', 'B', 'C', 'D', 'E', 'F', 0x7E}
	stream := append(frame1, frame2...)

	var frames [][]byte
	for _, b := range stream {
		f, err := parser.ProcessByte(b)
		if err != nil {
			t.Errorf("unexpected error in stream: %v", err)
		}
		if f != nil {
			frames = append(frames, f)
		}
	}

	if len(frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(frames))
	}
	if !bytes.Equal(frames[0], frame1) {
		t.Errorf("frame 1 mismatch")
	}
	if !bytes.Equal(frames[1], frame2) {
		t.Errorf("frame 2 mismatch")
	}
}
