package whatsmeow_service

import "testing"

func TestProxyRuntimeStatus(t *testing.T) {
	const id = "proxy-test"
	proxyRuntime.Delete(id)
	if _, ok := GetProxyRuntimeStatus(id); ok {
		t.Fatal("no status expected before the proxy is applied")
	}

	proxyEnabled(id)
	st, ok := GetProxyRuntimeStatus(id)
	if !ok || !st.RuntimeEnabled || st.FallbackWithoutProxy || st.LastAppliedAt == nil {
		t.Fatalf("unexpected status after enable: %#v", st)
	}

	proxyFailed(id, "proxy authentication failed; connected without proxy", true)
	st, _ = GetProxyRuntimeStatus(id)
	if st.RuntimeEnabled || !st.FallbackWithoutProxy || st.LastError == "" {
		t.Fatalf("unexpected status after fallback: %#v", st)
	}

	// A fresh successful application clears the previous failure.
	proxyEnabled(id)
	st, _ = GetProxyRuntimeStatus(id)
	if !st.RuntimeEnabled || st.FallbackWithoutProxy || st.LastError != "" {
		t.Fatalf("enable must reset the failure state: %#v", st)
	}
}
