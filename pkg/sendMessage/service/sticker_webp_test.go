package send_service

import (
	"encoding/binary"
	"testing"
)

// riffWebP builds a minimal RIFF/WEBP container with the given first chunk
// fourcc and (for VP8X) flags byte, padded so the declared size is consistent.
func riffWebP(chunk string, flags byte) []byte {
	payload := make([]byte, 0, 32)
	payload = append(payload, []byte("WEBP")...)
	payload = append(payload, []byte(chunk)...)
	payload = append(payload, 10, 0, 0, 0) // chunk size
	payload = append(payload, flags)
	payload = append(payload, make([]byte, 9)...)

	b := make([]byte, 8, 8+len(payload))
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:8], uint32(len(payload)))
	return append(b, payload...)
}

func TestIsWebP(t *testing.T) {
	if !isWebP(riffWebP("VP8 ", 0)) {
		t.Fatal("valid container should be accepted")
	}
	if isWebP([]byte("not webp at all")) {
		t.Fatal("garbage must be rejected")
	}
	truncated := riffWebP("VP8 ", 0)
	if isWebP(truncated[:len(truncated)-5]) {
		t.Fatal("a truncated download must be rejected")
	}
}

func TestWebpIsAnimated(t *testing.T) {
	if !webpIsAnimated(riffWebP("VP8X", 0x02)) {
		t.Fatal("VP8X with the animation flag should be animated")
	}
	if webpIsAnimated(riffWebP("VP8X", 0x00)) {
		t.Fatal("VP8X without the animation flag is static")
	}
	if webpIsAnimated(riffWebP("VP8 ", 0x02)) {
		t.Fatal("plain VP8 is always static")
	}
}
