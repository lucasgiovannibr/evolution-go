package utils

import (
	"net"
	"net/http"
	"time"
)

// Outbound HTTP used to go through http.Get / &http.Client{}, which have no timeout at
// all: a media URL, a link-preview site or web.whatsapp.com that accepts the connection
// and then stalls held the calling request forever (and, for the WhatsApp Web version
// lookup, a mutex that every instance start goes through).

func newClient(total time.Duration) *http.Client {
	base, _ := http.DefaultTransport.(*http.Transport)
	var t *http.Transport
	if base != nil {
		t = base.Clone()
	} else {
		t = &http.Transport{}
	}
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = 10 * time.Second
	// Time to the first byte of the answer; the body may take as long as the total.
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: t, Timeout: total}
}

// DownloadClient fetches user-supplied media (images, videos, documents by URL). The
// total limit is generous so that a large file on a slow link still completes.
var DownloadClient = newClient(5 * time.Minute)

// QuickClient is for small lookups (link previews, the WhatsApp Web version).
var QuickClient = newClient(15 * time.Second)
