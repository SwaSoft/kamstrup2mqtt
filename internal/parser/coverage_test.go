package parser

import "testing"

func feed(p *FrameParser, data []byte) (frames [][]byte, errs int) {
	for _, b := range data {
		f, err := p.ProcessByte(b)
		if err != nil {
			errs++
		}
		if f != nil {
			frames = append(frames, f)
		}
	}
	return
}

// mkFrame builds a frame of total size n (delimiters at both ends).
func mkFrame(n int) []byte {
	length := n - 2
	f := make([]byte, n)
	f[0] = Delimiter
	f[1] = byte(length >> 8 & 0x0F)
	f[2] = byte(length & 0xFF)
	for i := 3; i < n-1; i++ {
		f[i] = 0x01
	}
	f[n-1] = Delimiter
	return f
}

func TestFrameParser_SizeBoundaries(t *testing.T) {
	for _, n := range []int{MinPacketSize, MinPacketSize + 1, MaxPacketSize - 1, MaxPacketSize} {
		p := NewFrameParser()
		frames, errs := feed(p, mkFrame(n))
		if len(frames) != 1 || errs != 0 || len(frames[0]) != n {
			t.Errorf("size %d: frames=%d errs=%d", n, len(frames), errs)
		}
	}
	// just below min and just above max are rejected
	for _, length := range []int{MinPacketSize - 3, MaxPacketSize - 1} {
		p := NewFrameParser()
		_, errs := feed(p, []byte{Delimiter, byte(length >> 8 & 0x0F), byte(length)})
		if errs != 1 {
			t.Errorf("length %d: errs=%d", length, errs)
		}
	}
}

func TestFrameParser_IgnoresLeadingGarbageAndBackToBack(t *testing.T) {
	p := NewFrameParser()
	data := append([]byte{0x00, 0x01, 0xFF}, mkFrame(12)...)
	data = append(data, mkFrame(MinPacketSize)...)
	frames, errs := feed(p, data)
	if len(frames) != 2 || errs != 0 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_SplitDeliveryAndResetMidFrame(t *testing.T) {
	p := NewFrameParser()
	f := mkFrame(20)
	if frames, _ := feed(p, f[:7]); len(frames) != 0 {
		t.Fatal("no frame expected from partial data")
	}
	p.Reset()
	if p.frameStarted || p.byteCounter != 0 || p.expectedPacketSize != 0 || len(p.frameBuffer) != 0 {
		t.Errorf("state not cleared: %+v", p)
	}
	// remainder alone (no leading delimiter) must not yield a frame
	if frames, _ := feed(p, f[7:len(f)-1]); len(frames) != 0 {
		t.Error("unexpected frame")
	}
	// Reset on a fresh parser is harmless, and a full frame then parses
	p.Reset()
	if frames, errs := feed(p, f); len(frames) != 1 || errs != 0 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_ReturnedFrameIsCopy(t *testing.T) {
	p := NewFrameParser()
	frames, _ := feed(p, mkFrame(12))
	if len(frames) != 1 {
		t.Fatal("expected frame")
	}
	first := append([]byte(nil), frames[0]...)
	feed(p, mkFrame(12)[:5]) // reuses the internal buffer
	for i := range first {
		if frames[0][i] != first[i] {
			t.Fatal("returned frame aliased internal buffer")
		}
	}
}

func TestFrameParser_ErrorRecovery(t *testing.T) {
	p := NewFrameParser()
	bad := mkFrame(12)
	bad[11] = 0x00 // wrong terminator
	frames, errs := feed(p, bad)
	if len(frames) != 0 || errs != 1 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
	if frames, errs := feed(p, mkFrame(12)); len(frames) != 1 || errs != 0 {
		t.Errorf("no recovery: frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_ResyncOnEarlyDelimiter(t *testing.T) {
	p := NewFrameParser()
	data := append([]byte{Delimiter}, mkFrame(14)...)
	frames, errs := feed(p, data)
	if len(frames) != 1 || errs != 0 || len(frames[0]) != 14 {
		t.Errorf("frames=%d errs=%d", len(frames), errs)
	}
}

func TestFrameParser_Accessors(t *testing.T) {
	p := NewFrameParser()
	if p.FrameStarted() || p.ByteCounter() != 0 {
		t.Error("fresh parser should be idle")
	}
	feed(p, mkFrame(12)[:5])
	if !p.FrameStarted() || p.ByteCounter() != 5 {
		t.Errorf("started=%v counter=%d", p.FrameStarted(), p.ByteCounter())
	}
}
