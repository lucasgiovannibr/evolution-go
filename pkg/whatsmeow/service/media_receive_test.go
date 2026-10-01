package whatsmeow_service

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestPickMediaDescribesEachKind(t *testing.T) {
	cases := []struct {
		name      string
		msg       *waE2E.Message
		kind, ext string
		mime      string
		size      int64
		sticker   bool
	}{
		{"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{FileLength: proto.Uint64(10)}}, "image", ".jpg", "image/jpeg", 10, false},
		{"audio", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{FileLength: proto.Uint64(20)}}, "audio", ".ogg", "audio/ogg", 20, false},
		{"video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{FileLength: proto.Uint64(30)}}, "video", ".mp4", "video/mp4", 30, false},
		{"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{FileLength: proto.Uint64(40)}}, "sticker", ".png", "image/png", 40, true},
		{"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Mimetype: proto.String("application/pdf"), FileLength: proto.Uint64(50)}}, "document", getExtensionFromMimeType("application/pdf"), "application/pdf", 50, false},
		{"unknown size", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}, "image", ".jpg", "image/jpeg", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, ok := pickMedia(c.msg)
			if !ok {
				t.Fatal("media not found")
			}
			if m.kind != c.kind || m.extension != c.ext || m.mimeType != c.mime || m.size != c.size || m.sticker != c.sticker || m.child || m.target == nil {
				t.Fatalf("%+v", m)
			}
		})
	}
}

// The message itself wins over the child it carries, and among the kinds the order is the
// one the handler always had: image, audio, document, video, sticker.
func TestPickMediaPrecedence(t *testing.T) {
	both := &waE2E.Message{
		VideoMessage:           &waE2E.VideoMessage{},
		StickerMessage:         &waE2E.StickerMessage{},
		AssociatedChildMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}},
	}
	m, _ := pickMedia(both)
	if m.kind != "video" || m.child {
		t.Fatalf("got %+v", m)
	}
	m, _ = pickMedia(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{}, ImageMessage: &waE2E.ImageMessage{}})
	if m.kind != "image" {
		t.Fatalf("image is looked at first, got %s", m.kind)
	}
}

func TestPickMediaInAChildMessage(t *testing.T) {
	m, ok := pickMedia(&waE2E.Message{AssociatedChildMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}}})
	if !ok || !m.child || !m.sticker {
		t.Fatalf("%+v %v", m, ok)
	}
}

func TestPickMediaNothing(t *testing.T) {
	for _, m := range []*waE2E.Message{nil, {}, {Conversation: proto.String("hi")}, {AssociatedChildMessage: &waE2E.FutureProofMessage{}}} {
		if _, ok := pickMedia(m); ok {
			t.Fatalf("%+v has no media", m)
		}
	}
}
