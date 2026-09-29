package utils

import "testing"

// Every outbound client must carry a timeout: the ones it replaced had none.
func TestOutboundClientsHaveTimeouts(t *testing.T) {
	if DownloadClient.Timeout <= 0 || QuickClient.Timeout <= 0 {
		t.Fatal("clients must have an overall timeout")
	}
	if QuickClient.Timeout >= DownloadClient.Timeout {
		t.Fatal("the quick client must give up sooner than the download client")
	}
}
