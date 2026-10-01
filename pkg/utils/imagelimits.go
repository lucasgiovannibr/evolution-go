package utils

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os"
	"strconv"

	// decoders image.DecodeConfig can use
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// An image is cheap to send and expensive to decode: a PNG of a few KB can declare
// 30000x30000 pixels and ask the decoder for gigabytes. Thumbnails, sticker conversion and
// the preview of a received sticker decoded whatever arrived, so a single crafted file (from
// a caller, from a URL, or from any WhatsApp contact for a sticker) could take the process
// down.
//
// DecodeConfig reads only the header, so the size is checked first.

// DefaultMaxImagePixels is the largest image (width x height) that is decoded: 50
// megapixels, well above any real photo and far below what exhausts memory.
const DefaultMaxImagePixels = 50_000_000

// MaxStickerPixels is the limit for stickers (WhatsApp's are 512x512).
const MaxStickerPixels = 4_000_000

// ErrImageTooLarge: the image declares more pixels than may be decoded.
var ErrImageTooLarge = errors.New("image dimensions are too large to process")

// MaxImagePixels is the configured limit (MAX_IMAGE_MEGAPIXELS, default 50).
func MaxImagePixels() int {
	if mp, err := strconv.Atoi(os.Getenv("MAX_IMAGE_MEGAPIXELS")); err == nil && mp > 0 {
		return mp * 1_000_000
	}
	return DefaultMaxImagePixels
}

// CheckImageDimensions fails when data is an image that declares more than maxPixels
// pixels, or when its header cannot be read. It does not decode the image.
func CheckImageDimensions(data []byte, maxPixels int) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("not a readable image: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return errors.New("image has no pixels")
	}
	// int64: width*height of a crafted header can overflow an int on 32-bit platforms.
	if int64(cfg.Width)*int64(cfg.Height) > int64(maxPixels) {
		return fmt.Errorf("%w: %dx%d", ErrImageTooLarge, cfg.Width, cfg.Height)
	}
	return nil
}
