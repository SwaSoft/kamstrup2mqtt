package main

import (
	"fmt"
)

// Frame-related constants
const (
	Delimiter     = 0x7E
	MinPacketSize = 10
	MaxPacketSize = 4097
)

// FrameParser handles the state machine for parsing serial frames.
type FrameParser struct {
	frameStarted       bool
	frameBuffer        []byte
	byteCounter        int
	expectedPacketSize int
}

func NewFrameParser() *FrameParser {
	return &FrameParser{
		frameBuffer: make([]byte, 0, MaxPacketSize),
	}
}

// Reset clears the current parsing state.
func (p *FrameParser) Reset() {
	p.frameStarted = false
	p.frameBuffer = p.frameBuffer[:0]
	p.byteCounter = 0
	p.expectedPacketSize = 0
}

// ProcessByte handles an incoming byte and returns a complete frame if one is found.
func (p *FrameParser) ProcessByte(b byte) ([]byte, error) {
	if !p.frameStarted {
		if b == Delimiter {
			p.frameStarted = true
			p.frameBuffer = append(p.frameBuffer[:0], b)
			p.byteCounter = 1
			p.expectedPacketSize = 0
		}
		return nil, nil
	}

	p.frameBuffer = append(p.frameBuffer, b)
	p.byteCounter++

	if b == Delimiter {
		if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
			frame := make([]byte, len(p.frameBuffer))
			copy(frame, p.frameBuffer)
			p.Reset()
			return frame, nil
		}

		if p.byteCounter < 3 {
			p.Reset()
			p.frameStarted = true
			p.frameBuffer = append(p.frameBuffer[:0], b)
			p.byteCounter = 1
			return nil, nil
		}
	}

	if p.byteCounter == 3 {
		length := (int(p.frameBuffer[1]&0x0F) << 8) | int(p.frameBuffer[2])
		p.expectedPacketSize = length + 2
		if p.expectedPacketSize < MinPacketSize || p.expectedPacketSize > MaxPacketSize {
			actualSize := p.expectedPacketSize
			p.Reset()
			return nil, fmt.Errorf("invalid packet size %d (length=%d)", actualSize, length)
		}
	}

	if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
		p.Reset()
		return nil, fmt.Errorf("malformed frame: last byte at index %d is 0x%02X not 0x%02X", p.byteCounter-1, b, Delimiter)
	}

	return nil, nil
}
