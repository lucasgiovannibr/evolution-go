package utils

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---- images ----

func realPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// craftedPNG is a few hundred bytes that declare w x h pixels: only the header is real, so
// a decoder that trusts it allocates w*h*4 bytes.
func craftedPNG(w, h uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // RGBA
	chunk := func(kind string, data []byte) {
		var lenb [4]byte
		binary.BigEndian.PutUint32(lenb[:], uint32(len(data)))
		b.Write(lenb[:])
		b.WriteString(kind)
		b.Write(data)
		crc := crc32.NewIEEE()
		crc.Write([]byte(kind))
		crc.Write(data)
		var crcb [4]byte
		binary.BigEndian.PutUint32(crcb[:], crc.Sum32())
		b.Write(crcb[:])
	}
	chunk("IHDR", ihdr)
	chunk("IEND", nil)
	return b.Bytes()
}

func TestCheckImageDimensions(t *testing.T) {
	if err := CheckImageDimensions(realPNG(t, 640, 480), DefaultMaxImagePixels); err != nil {
		t.Fatalf("a normal image must pass: %v", err)
	}

	// 30000x30000 = 900 megapixels = 3.6 GB once decoded, declared by ~70 bytes.
	crafted := craftedPNG(30000, 30000)
	if len(crafted) > 100 {
		t.Fatalf("the crafted file is meant to be tiny, got %d bytes", len(crafted))
	}
	err := CheckImageDimensions(crafted, DefaultMaxImagePixels)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("the decompression bomb must be refused, got %v", err)
	}

	// the limit is on the product, not on each side
	if err := CheckImageDimensions(craftedPNG(10000, 4000), DefaultMaxImagePixels); err != nil {
		t.Fatalf("40 megapixels is within the 50 megapixel limit: %v", err)
	}
	if err := CheckImageDimensions(craftedPNG(10000, 6000), DefaultMaxImagePixels); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("60 megapixels is over the limit, got %v", err)
	}

	// even with sides that would overflow a 32-bit product
	if err := CheckImageDimensions(craftedPNG(1<<30, 4), DefaultMaxImagePixels); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("an overflowing product must be refused, got %v", err)
	}

	for name, data := range map[string][]byte{"not an image": []byte("hello"), "empty": nil} {
		if err := CheckImageDimensions(data, DefaultMaxImagePixels); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestStickerLimitIsTighter(t *testing.T) {
	if err := CheckImageDimensions(realPNG(t, 512, 512), MaxStickerPixels); err != nil {
		t.Fatalf("a real sticker is 512x512: %v", err)
	}
	if err := CheckImageDimensions(craftedPNG(4000, 4000), MaxStickerPixels); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("16 megapixels is too much for a sticker, got %v", err)
	}
}

func TestMaxImagePixelsFromTheEnvironment(t *testing.T) {
	if MaxImagePixels() != DefaultMaxImagePixels {
		t.Fatal("default is 50 megapixels")
	}
	t.Setenv("MAX_IMAGE_MEGAPIXELS", "12")
	if MaxImagePixels() != 12_000_000 {
		t.Fatalf("got %d", MaxImagePixels())
	}
	t.Setenv("MAX_IMAGE_MEGAPIXELS", "nonsense")
	if MaxImagePixels() != DefaultMaxImagePixels {
		t.Fatal("an invalid value falls back to the default")
	}
}

// ---- external programs ----

func needs(t *testing.T, programs ...string) {
	t.Helper()
	for _, p := range programs {
		if _, err := exec.LookPath(p); err != nil {
			t.Skipf("%s is not available", p)
		}
	}
}

func TestRunLimitedFeedsStdinAndReturnsStdout(t *testing.T) {
	needs(t, "cat")
	out, _, err := RunLimited(context.Background(), 5*time.Second, "cat", nil, []byte("hello"), 1<<20)
	if err != nil || string(out) != "hello" {
		t.Fatalf("out %q err %v", out, err)
	}
}

// pdftoppm had no time limit at all.
func TestRunLimitedKillsAProgramThatRunsTooLong(t *testing.T) {
	needs(t, "sleep")
	start := time.Now()
	_, _, err := RunLimited(context.Background(), 200*time.Millisecond, "sleep", []string{"30"}, nil, 1<<20)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want a timeout, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("the program was not killed: took %v", time.Since(start))
	}
}

func TestRunLimitedCapsTheOutput(t *testing.T) {
	needs(t, "sh", "yes")
	start := time.Now()
	_, _, err := RunLimited(context.Background(), 20*time.Second, "yes", nil, nil, 64<<10)
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("a program that never stops writing must hit the cap, got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("the cap did not stop the program quickly: %v", time.Since(start))
	}
}

func TestRunLimitedKeepsTheTailOfStderr(t *testing.T) {
	needs(t, "sh")
	// 200 KB on stderr; the last line is what matters (ffmpeg prints the duration last)
	script := `i=0; while [ $i -lt 4000 ]; do echo "progress line $i padding padding padding padding padding" 1>&2; i=$((i+1)); done; echo "time=00:01:02.50" 1>&2`
	_, errOut, err := RunLimited(context.Background(), 20*time.Second, "sh", []string{"-c", script}, nil, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(errOut) > stderrKeep {
		t.Fatalf("stderr kept %d bytes, the limit is %d", len(errOut), stderrKeep)
	}
	if !strings.Contains(string(errOut), "time=00:01:02.50") {
		t.Fatal("the tail of stderr must be kept")
	}
}

func TestRunLimitedReportsTheExitErrorWithStderr(t *testing.T) {
	needs(t, "sh")
	_, errOut, err := RunLimited(context.Background(), 5*time.Second, "sh", []string{"-c", "echo broken input >&2; exit 3"}, nil, 1<<20)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !strings.Contains(string(errOut), "broken input") {
		t.Fatalf("err %v stderr %q", err, errOut)
	}
}

// Ten concurrent audio sends used to be ten ffmpeg processes: the pool bounds them.
func TestTheSlotPoolBoundsConcurrencyAndTimesOut(t *testing.T) {
	needs(t, "sleep")
	SetMaxConcurrentConversions(1)
	defer SetMaxConcurrentConversions(defaultSlots())

	// One long job holds the only slot...
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		RunLimited(context.Background(), 3*time.Second, "sleep", []string{"1"}, nil, 1<<10)
	}()
	<-started
	time.Sleep(100 * time.Millisecond)

	// ...so a second one cannot start until the context gives up (the real wait is 30 s).
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err := RunLimited(ctx, 3*time.Second, "sleep", []string{"1"}, nil, 1<<10)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("with every slot taken the second job must wait, then give up: %v", err)
	}
	<-done

	// once the slot is free it runs
	if _, _, err := RunLimited(context.Background(), 3*time.Second, "sleep", []string{"0"}, nil, 1<<10); err != nil {
		t.Fatalf("after the first job the slot is free: %v", err)
	}
}

func TestLimitedBufferAndTailBuffer(t *testing.T) {
	l := &limitedBuffer{max: 10}
	if n, err := l.Write([]byte("12345")); n != 5 || err != nil {
		t.Fatal(n, err)
	}
	if _, err := l.Write([]byte("678901")); !errors.Is(err, ErrOutputTooLarge) || !l.exceeded {
		t.Fatalf("exceeding the cap must fail and be remembered: %v", err)
	}

	tail := &tailBuffer{n: 5}
	for _, chunk := range []string{"abc", "defgh", "ijklmnop"} {
		tail.Write([]byte(chunk))
	}
	if string(tail.Bytes()) != "lmnop" {
		t.Fatalf("tail = %q", tail.Bytes())
	}
}
