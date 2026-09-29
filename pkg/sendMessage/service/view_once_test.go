package send_service

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestApplyViewOnce(t *testing.T) {
	img := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
	applyViewOnce(img, true)
	if !img.ImageMessage.GetViewOnce() {
		t.Fatal("image should be flagged view-once")
	}

	vid := &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}
	applyViewOnce(vid, true)
	if !vid.VideoMessage.GetViewOnce() {
		t.Fatal("video should be flagged view-once")
	}

	aud := &waE2E.Message{AudioMessage: &waE2E.AudioMessage{}}
	applyViewOnce(aud, true)
	if !aud.AudioMessage.GetViewOnce() {
		t.Fatal("audio should be flagged view-once")
	}

	off := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
	applyViewOnce(off, false)
	if off.ImageMessage.ViewOnce != nil {
		t.Fatal("disabled flag must leave the message untouched")
	}

	doc := &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}}
	applyViewOnce(doc, true) // documents do not support it; must not panic or change anything

	applyViewOnce(nil, true)
}
