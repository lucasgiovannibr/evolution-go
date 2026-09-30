package call_stream

import (
	"bytes"
	"testing"
)

// nal builds one NAL unit of the given type with a payload byte pattern.
func nal(typ byte, payload ...byte) []byte {
	return append([]byte{0x60 | typ}, payload...) // nal_ref_idc 3
}

func annexB(short bool, units ...[]byte) []byte {
	var out []byte
	for i, u := range units {
		if short && i > 0 {
			out = append(out, startCode3...)
		} else {
			out = append(out, startCode4...)
		}
		out = append(out, u...)
	}
	return out
}

var (
	sps   = nal(nalSPS, 0x42, 0x00, 0x1f)
	pps   = nal(nalPPS, 0xce, 0x3c)
	idr   = nal(nalIDR, 0x88, 0x84, 0x00, 0x33)
	slice = nal(1, 0x9a, 0x24)
	aud   = nal(nalAUD, 0xf0)
	sei   = nal(6, 0x05, 0x10)
)

func TestNALUnitsSplitsBothStartCodeLengths(t *testing.T) {
	units := nalUnits(annexB(false, sps, pps, idr))
	if len(units) != 3 || !bytes.Equal(units[0], sps) || !bytes.Equal(units[1], pps) || !bytes.Equal(units[2], idr) {
		t.Fatalf("units = %x", units)
	}

	// three-byte start codes after the first: the zero of a four-byte code must not stick to the NAL before it
	units = nalUnits(annexB(true, sps, pps, idr))
	if len(units) != 3 || !bytes.Equal(units[2], idr) {
		t.Fatalf("units = %x", units)
	}
	mixed := append(append(append([]byte{}, startCode3...), slice...), append(startCode4, idr...)...)
	units = nalUnits(mixed)
	if len(units) != 2 || !bytes.Equal(units[0], slice) || !bytes.Equal(units[1], idr) {
		t.Fatalf("mixed: units = %x", units)
	}
}

func TestNALUnitsRefusesWhatIsNotAnnexB(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":           nil,
		"AVCC length":     {0, 0, 0, 5, 0x65, 1, 2, 3, 4}, // 4-byte length prefix, as in MP4
		"raw NAL":         idr,
		"garbage":         []byte("hello"),
		"start code only": startCode4,
	} {
		if units := nalUnits(data); len(units) != 0 {
			t.Errorf("%s: units = %x, want none", name, units)
		}
	}
}

func TestIsKeyframe(t *testing.T) {
	if !isKeyframe(annexB(false, sps, pps, idr)) {
		t.Error("an access unit with an IDR is a keyframe")
	}
	if isKeyframe(annexB(false, slice)) {
		t.Error("a P slice is not a keyframe")
	}
	if isKeyframe([]byte("nonsense")) || isKeyframe(nil) {
		t.Error("data that is not H.264 is not a keyframe")
	}
}

func TestAKeyframeThatLacksTheHeadersGetsTheOnesTheClientSentBefore(t *testing.T) {
	var h headerRepair

	first, ok := h.repair(annexB(false, sps, pps, idr))
	if !ok || !bytes.Equal(first, annexB(false, sps, pps, idr)) {
		t.Fatalf("a keyframe that has its headers must be sent as it is: %x", first)
	}
	if p, _ := h.repair(annexB(false, slice)); !bytes.Equal(p, annexB(false, slice)) {
		t.Fatal("a P slice must not be touched")
	}

	// later, on WhatsApp's request, the encoder sends a bare IDR
	repaired, ok := h.repair(annexB(false, idr))
	if !ok {
		t.Fatal("refused a bare IDR")
	}
	if want := annexB(false, sps, pps, idr); !bytes.Equal(repaired, want) {
		t.Fatalf("repaired = %x, want %x", repaired, want)
	}
}

func TestRepairKeepsTheDelimiterFirstAndDoesNotDuplicateHeaders(t *testing.T) {
	var h headerRepair
	h.repair(annexB(false, sps, pps, idr))

	// an AUD and an SEI, and only the SPS: the PPS is missing
	repaired, _ := h.repair(annexB(false, aud, sei, sps, idr))

	want := annexB(false, aud, sps, pps, sei, idr)
	if !bytes.Equal(repaired, want) {
		t.Fatalf("repaired = %x\n     want = %x", repaired, want)
	}
}

func TestRepairWithoutKnownHeadersLeavesTheKeyframeAlone(t *testing.T) {
	var h headerRepair
	bare := annexB(false, idr)

	out, ok := h.repair(bare)

	if !ok || !bytes.Equal(out, bare) {
		t.Fatalf("out = %x, ok = %v: with nothing to add the keyframe goes as it is", out, ok)
	}
}

func TestRepairUsesTheLatestHeaders(t *testing.T) {
	var h headerRepair
	h.repair(annexB(false, sps, pps, idr))
	newSPS := nal(nalSPS, 0x64, 0x00, 0x28)
	h.repair(annexB(false, newSPS, pps, idr)) // the encoder was reconfigured

	out, _ := h.repair(annexB(false, idr))

	if !bytes.Equal(out, annexB(false, newSPS, pps, idr)) {
		t.Fatalf("out = %x: must use the SPS that came last", out)
	}
}

func TestRepairRefusesWhatIsNotAnnexB(t *testing.T) {
	var h headerRepair
	if _, ok := h.repair([]byte{0, 0, 0, 5, 0x65, 1, 2, 3, 4}); ok {
		t.Fatal("accepted AVCC framing")
	}
	if _, ok := h.repair(nil); ok {
		t.Fatal("accepted nothing")
	}
}
