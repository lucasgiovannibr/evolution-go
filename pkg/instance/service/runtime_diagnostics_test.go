package instance_service

import (
	"testing"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
)

func codesOf(d RuntimeDiagnostics) map[string]bool {
	m := map[string]bool{}
	for _, w := range d.Warnings {
		m[w.Code] = true
	}
	return m
}

func rt(id string, mod func(*whatsmeow_service.RuntimeInfo)) whatsmeow_service.RuntimeInfo {
	r := whatsmeow_service.RuntimeInfo{InstanceID: id, Warnings: []whatsmeow_service.Warning{}}
	if mod != nil {
		mod(&r)
	}
	return r
}

func TestDiagnoseConsistentInstance(t *testing.T) {
	inst := &instance_model.Instance{Id: "a", Name: "a", Connected: true, Jid: "5511:1@s.whatsapp.net"}
	d := diagnose(inst, rt("a", func(r *whatsmeow_service.RuntimeInfo) {
		r.ClientRegistered, r.RuntimeActive, r.KillChannel, r.SupervisorCurrent = true, true, true, true
		r.WebsocketConnected, r.LoggedIn, r.DeviceJID = true, true, "5511:1@s.whatsapp.net"
	}))
	if len(d.Warnings) != 0 {
		t.Fatalf("a consistent instance must have no warnings: %v", d.Warnings)
	}
}

// The state seen in the first live test: paired in the database, but the running
// client was a brand-new unpaired device and the database said "disconnected".
func TestDiagnoseLostSessionAfterDuplicateRuntime(t *testing.T) {
	inst := &instance_model.Instance{Id: "a", Connected: false, Jid: "5511:1@s.whatsapp.net"}
	d := diagnose(inst, rt("a", func(r *whatsmeow_service.RuntimeInfo) {
		r.ClientRegistered, r.RuntimeActive, r.KillChannel, r.SupervisorCurrent = true, true, true, true
		r.WebsocketConnected = true // connected, but as a new device: no JID, not logged in
	}))
	if !codesOf(d)["paired_in_db_unpaired_runtime"] {
		t.Fatalf("got %v", d.Warnings)
	}
}

func TestDiagnoseMismatches(t *testing.T) {
	up := rt("a", func(r *whatsmeow_service.RuntimeInfo) {
		r.ClientRegistered, r.RuntimeActive, r.KillChannel, r.SupervisorCurrent = true, true, true, true
		r.WebsocketConnected, r.LoggedIn, r.DeviceJID = true, true, "5511:1@s.whatsapp.net"
	})
	if c := codesOf(diagnose(&instance_model.Instance{Id: "a", Connected: false, Jid: "5511:1@s.whatsapp.net"}, up)); !c["db_disconnected_runtime_online"] {
		t.Fatalf("got %v", c)
	}

	down := rt("a", func(r *whatsmeow_service.RuntimeInfo) {
		r.ClientRegistered, r.RuntimeActive, r.KillChannel, r.SupervisorCurrent = true, true, true, true
		r.ReconnectInProgress = true
	})
	if c := codesOf(diagnose(&instance_model.Instance{Id: "a", Connected: true}, down)); !c["db_connected_runtime_offline"] {
		t.Fatalf("got %v", c)
	}

	if c := codesOf(diagnose(&instance_model.Instance{Id: "a", Jid: "5511:1@s.whatsapp.net"}, rt("a", nil))); !c["paired_without_runtime"] {
		t.Fatalf("got %v", c)
	}
}

func TestBuildReportIncludesRuntimesOfDeletedInstances(t *testing.T) {
	rows := []*instance_model.Instance{{Id: "a", Name: "a"}}
	runtimes := []whatsmeow_service.RuntimeInfo{
		rt("a", nil),
		rt("ghost", func(r *whatsmeow_service.RuntimeInfo) { r.ClientRegistered, r.RuntimeActive = true, true }),
	}

	report := buildReport(rows, runtimes, whatsmeow_service.ProcessInfo{Goroutines: 10})
	if len(report.Instances) != 2 || report.Summary.Instances != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}

	ghost := report.Instances[1]
	if ghost.InstanceID != "ghost" || ghost.Database != nil || !codesOf(ghost)["runtime_for_deleted_instance"] {
		t.Fatalf("a runtime without a database row must be flagged: %#v", ghost)
	}
	if report.Summary.WithWarnings != 1 || report.Process.Goroutines != 10 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
}
