package whatsmeow_service

import (
	"context"
	"encoding/base64"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// downloadTimeout bounds one download: large videos are slow, but a stalled one must not
// hold a media worker for ever.
const downloadTimeout = 5 * time.Minute

// receivedMedia is the file of a received message: what to download and how to describe it.
type receivedMedia struct {
	target    whatsmeow.DownloadableMessage
	kind      string // "image", "audio", "document", "video", "sticker"
	extension string
	mimeType  string
	size      int64 // announced size, 0 when unknown
	sticker   bool
	child     bool // it is inside an associated child message (a reply with media)
}

// pickMedia returns the media of a message, looking at the message itself first and then at
// the child message it carries, in the order image, audio, document, video, sticker.
func pickMedia(m *waE2E.Message) (receivedMedia, bool) {
	if r, ok := mediaOf(m); ok {
		return r, true
	}
	if child := m.GetAssociatedChildMessage().GetMessage(); child != nil {
		if r, ok := mediaOf(child); ok {
			r.child = true
			return r, true
		}
	}
	return receivedMedia{}, false
}

func mediaOf(m *waE2E.Message) (receivedMedia, bool) {
	switch {
	case m.GetImageMessage() != nil:
		img := m.GetImageMessage()
		return receivedMedia{target: img, kind: "image", extension: ".jpg", mimeType: "image/jpeg", size: int64(img.GetFileLength())}, true
	case m.GetAudioMessage() != nil:
		a := m.GetAudioMessage()
		return receivedMedia{target: a, kind: "audio", extension: ".ogg", mimeType: "audio/ogg", size: int64(a.GetFileLength())}, true
	case m.GetDocumentMessage() != nil:
		d := m.GetDocumentMessage()
		return receivedMedia{target: d, kind: "document", extension: getExtensionFromMimeType(d.GetMimetype()), mimeType: d.GetMimetype(), size: int64(d.GetFileLength())}, true
	case m.GetVideoMessage() != nil:
		v := m.GetVideoMessage()
		return receivedMedia{target: v, kind: "video", extension: ".mp4", mimeType: "video/mp4", size: int64(v.GetFileLength())}, true
	case m.GetStickerMessage() != nil:
		s := m.GetStickerMessage()
		return receivedMedia{target: s, kind: "sticker", extension: ".png", mimeType: "image/png", size: int64(s.GetFileLength()), sticker: true}, true
	}
	return receivedMedia{}, false
}

// attachMedia downloads the media of a received message and puts it in the event data: an S3
// URL when object storage is configured, base64 otherwise. A failure is logged and the event
// goes on without the file.
func (mycli *MyClient) attachMedia(evt *events.Message, dataMap map[string]interface{}) {
	media, ok := pickMedia(evt.Message)
	if !ok {
		return
	}
	log := mycli.loggerWrapper.GetLogger(mycli.userID)
	id := evt.Info.ID
	log.LogInfo("[%s] Processing media message - ID: %s (%s, child message: %v, %d bytes announced)", mycli.userID, id, media.kind, media.child, media.size)

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	start := time.Now()

	data, err := mycli.WAClient.Download(ctx, media.target)
	if err == nil && media.sticker {
		if png, convErr := stickerAsPNG(data); convErr != nil {
			log.LogWarn("[%s] Could not convert the sticker to PNG, keeping raw webp: %v", mycli.userID, convErr)
			media.extension, media.mimeType = ".webp", "image/webp"
		} else {
			data = png
		}
	}
	took := time.Since(start)

	if err != nil {
		log.LogError("[%s] Failed to download media - ID: %s, Size: %d bytes, Duration: %v, Error: %v", mycli.userID, id, media.size, took, err)
		if ctx.Err() == context.DeadlineExceeded {
			log.LogError("[%s] Download timeout exceeded (%v) - ID: %s, Size: %d bytes", mycli.userID, downloadTimeout, id, media.size)
		}
		log.LogWarn("[%s] Continuing message processing without media - ID: %s", mycli.userID, id)
		return
	}
	if len(data) == 0 {
		log.LogWarn("[%s] Skipping media storage: empty download - ID: %s", mycli.userID, id)
		return
	}
	log.LogInfo("[%s] Media download successful - ID: %s, Expected: %d bytes, Actual: %d bytes, Duration: %v", mycli.userID, id, media.size, len(data), took)
	if media.size > 0 && int64(len(data)) != media.size {
		log.LogWarn("[%s] Size mismatch detected - ID: %s, Expected: %d, Got: %d", mycli.userID, id, media.size, len(data))
	}

	messageMap, ok := dataMap["Message"].(map[string]interface{})
	if !ok {
		messageMap = make(map[string]interface{})
	}

	if mycli.config.MinioEnabled {
		fileName := id + media.extension
		storeStart := time.Now()
		log.LogInfo("[%s] Uploading to S3/Minio - ID: %s, FileName: %s, Size: %d bytes", mycli.userID, id, fileName, len(data))
		mediaURL, err := mycli.mediaStorage.Store(context.Background(), mycli.userID, data, fileName, media.mimeType)
		if err != nil {
			log.LogError("[%s] Failed to store media in S3/Minio - ID: %s, Size: %d bytes, Duration: %v, Error: %v", mycli.userID, id, len(data), time.Since(storeStart), err)
			log.LogWarn("[%s] Continuing message processing without S3 URL - ID: %s", mycli.userID, id)
		} else {
			log.LogInfo("[%s] S3/Minio upload successful - ID: %s, Size: %d bytes, Duration: %v, URL: %s", mycli.userID, id, len(data), time.Since(storeStart), mediaURL)
			messageMap["mediaUrl"] = mediaURL
			messageMap["mimetype"] = media.mimeType
		}
	} else {
		encodeStart := time.Now()
		encoded := base64.StdEncoding.EncodeToString(data)
		log.LogInfo("[%s] Base64 encoding completed - ID: %s, Original: %d bytes, Encoded: %d chars, Duration: %v", mycli.userID, id, len(data), len(encoded), time.Since(encodeStart))
		messageMap["base64"] = encoded
	}
	dataMap["Message"] = messageMap
}
