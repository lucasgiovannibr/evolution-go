package send_service

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"runtime"
	"testing"
)

// craftedPNG declares w x h pixels in a header of ~70 bytes and has no pixel data.
func craftedPNG(w, h uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6
	chunk := func(kind string, data []byte) {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(data)))
		b.Write(l[:])
		b.WriteString(kind)
		b.Write(data)
		c := crc32.NewIEEE()
		c.Write([]byte(kind))
		c.Write(data)
		var s [4]byte
		binary.BigEndian.PutUint32(s[:], c.Sum32())
		b.Write(s[:])
	}
	chunk("IHDR", ihdr)
	chunk("IEND", nil)
	return b.Bytes()
}

func allocated() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.TotalAlloc
}

// A 30000x30000 image would take 3.6 GB to decode; the thumbnail is skipped instead.
func TestThumbnailOfADecompressionBombIsSkippedWithoutDecoding(t *testing.T) {
	bomb := craftedPNG(30000, 30000)

	before := allocated()
	if thumb := makeJPEGThumbnail(bomb, 72); thumb != nil {
		t.Fatal("no thumbnail must be made from an image declaring 900 megapixels")
	}
	if grew := allocated() - before; grew > 32<<20 {
		t.Fatalf("the check allocated %d MB: the image was decoded", grew>>20)
	}
}

func TestStickerFromAURLIsRefusedWhenItDeclaresTooManyPixels(t *testing.T) {
	_, err := stickerFromBytes(craftedPNG(30000, 30000))
	if err == nil {
		t.Fatal("a sticker image declaring 900 megapixels must be refused")
	}
}
