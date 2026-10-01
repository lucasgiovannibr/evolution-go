package send_service

import "testing"

// The shape below is the one WhatsApp accepted and rendered (phone and WhatsApp Web) for reply and
// CTA buttons from a Business account; see SendButton.
func TestNativeFlowBizNodes(t *testing.T) {
	nodes := nativeFlowBizNodes("mixed", "9", "5531999990000")
	if len(nodes) != 2 || nodes[0].Tag != "biz" || nodes[1].Tag != "bot" {
		t.Fatalf("a 1:1 chat needs <biz> and <bot>, got %+v", nodes)
	}
	if got := nodes[1].Attrs["biz_bot"]; got != "1" {
		t.Fatalf("biz_bot = %v", got)
	}

	if group := nativeFlowBizNodes("mixed", "9", "120363000000000000@g.us"); len(group) != 1 || group[0].Tag != "biz" {
		t.Fatalf("a group takes no <bot>, got %+v", group)
	}
}

// An empty title/subtitle on the card header (the field present, with no text) made the iPhone
// drop the carousel while WhatsApp Web still showed it.
func TestCarouselCardHeaderLeavesEmptyTextOut(t *testing.T) {
	bare := carouselCardHeader(CarouselCardHeaderStruct{})
	if bare.Title != nil || bare.Subtitle != nil {
		t.Fatalf("empty title/subtitle must be left out, got %v / %v", bare.Title, bare.Subtitle)
	}
	if bare.GetHasMediaAttachment() {
		t.Fatal("no media attached yet")
	}

	full := carouselCardHeader(CarouselCardHeaderStruct{Title: "T", Subtitle: "S"})
	if full.GetTitle() != "T" || full.GetSubtitle() != "S" {
		t.Fatalf("text must be kept, got %q / %q", full.GetTitle(), full.GetSubtitle())
	}
}
