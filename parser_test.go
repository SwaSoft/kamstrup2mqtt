package main

import (
	"bytes"
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

func TestFrameParser_Resync(t *testing.T) {
	parser := &FrameParser{}
	// Garbage data followed by a sudden new 0x7E delimiter
	garbage := []byte{0x7E, 0x00, 0x08, '1', '2', '3', '4', '5'} // Partial
	payload2 := []byte{0x7E, 0x00, 0x08, 'A', 'B', 'C', 'D', 'E', 'F', 0x7E}

	for _, b := range garbage {
		parser.ProcessByte(b)
	}

	// Now suddenly a new 0x7E arrives instead of '6'.
	frame, err := parser.ProcessByte(0x7E)
	if err == nil {
		t.Error("expected error when 0x7E appears early")
	}
	if frame != nil {
		t.Error("expected no frame")
	}

	// Should NOT have reset in a way that ignores this 0x7E
	var lastFrame []byte
	for _, b := range payload2[1:] { // Skip the 0x7E we just processed
		f, _ := parser.ProcessByte(b)
		if f != nil {
			lastFrame = f
		}
	}

	if !bytes.Equal(lastFrame, payload2) {
		t.Errorf("expected frame %v, got %v", payload2, lastFrame)
	}
}
