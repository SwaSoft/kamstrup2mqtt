// Package parser implements serial frame parsing. It is pure and performs no I/O.
package parser

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

// FrameStarted reports whether a frame is currently being assembled.
func (p *FrameParser) FrameStarted() bool {
	return p.frameStarted
}

// ByteCounter returns the number of bytes received in the current frame.
func (p *FrameParser) ByteCounter() int {
	return p.byteCounter
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
			p.startFrame(b)
		}
		return nil, nil
	}

	p.frameBuffer = append(p.frameBuffer, b)
	p.byteCounter++

	// Resync on delimiter before length field is complete
	if b == Delimiter && p.byteCounter < 3 {
		p.startFrame(b)
		return nil, nil
	}

	// Parse length field at byte 3
	if p.byteCounter == 3 {
		length := int(p.frameBuffer[1]&0x0F)<<8 | int(p.frameBuffer[2])
		p.expectedPacketSize = length + 2
		if p.expectedPacketSize < MinPacketSize || p.expectedPacketSize > MaxPacketSize {
			size := p.expectedPacketSize
			p.Reset()
			return nil, fmt.Errorf("invalid packet size %d (length=%d)", size, length)
		}
	}

	// Check for frame completion
	if p.expectedPacketSize > 0 && p.byteCounter == p.expectedPacketSize {
		if b == Delimiter {
			frame := make([]byte, len(p.frameBuffer))
			copy(frame, p.frameBuffer)
			p.Reset()
			return frame, nil
		}
		idx := p.byteCounter - 1
		p.Reset()
		return nil, fmt.Errorf("malformed frame: last byte at index %d is 0x%02X not 0x%02X", idx, b, Delimiter)
	}

	return nil, nil
}

// startFrame initializes state for a new frame starting with the given delimiter byte.
func (p *FrameParser) startFrame(b byte) {
	p.frameStarted = true
	p.frameBuffer = p.frameBuffer[:0]
	p.frameBuffer = append(p.frameBuffer, b)
	p.byteCounter = 1
	p.expectedPacketSize = 0
}
