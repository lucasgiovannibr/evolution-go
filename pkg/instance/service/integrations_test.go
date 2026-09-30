package instance_service

import (
	"strings"
	"testing"
)

func TestIntegrationsValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      IntegrationsStruct
		wantErr string // substring; empty means valid
	}{
		{name: "empty body is valid (keeps everything)", in: IntegrationsStruct{}},
		{name: "https webhook", in: IntegrationsStruct{WebhookUrl: "https://hooks.example.com/wa"}},
		{name: "http webhook with port", in: IntegrationsStruct{WebhookUrl: "http://localhost:3000/hook"}},
		{name: "disabled removes the webhook", in: IntegrationsStruct{WebhookUrl: "disabled"}},
		{name: "false removes the webhook", in: IntegrationsStruct{WebhookUrl: "false"}},
		{name: "webhook without scheme", in: IntegrationsStruct{WebhookUrl: "hooks.example.com"}, wantErr: "webhookUrl"},
		{name: "webhook with other scheme", in: IntegrationsStruct{WebhookUrl: "ftp://x/y"}, wantErr: "webhookUrl"},
		{name: "webhook without host", in: IntegrationsStruct{WebhookUrl: "https://"}, wantErr: "webhookUrl"},
		{name: "known events", in: IntegrationsStruct{Subscribe: []string{"MESSAGE", "SEND_MESSAGE", "USER_ABOUT"}}},
		{name: "ALL alone", in: IntegrationsStruct{Subscribe: []string{"ALL"}}},
		{name: "ALL mixed with others", in: IntegrationsStruct{Subscribe: []string{"MESSAGE", "ALL"}}, wantErr: "ALL"},
		{name: "unknown events are named", in: IntegrationsStruct{Subscribe: []string{"MESSAGE", "NOPE", "ALSO_NOPE"}}, wantErr: "NOPE, ALSO_NOPE"},
		{name: "lowercase event is unknown", in: IntegrationsStruct{Subscribe: []string{"message"}}, wantErr: "message"},
		{name: "producers enabled and disabled", in: IntegrationsStruct{RabbitmqEnable: "enabled", WebSocketEnable: "disabled", NatsEnable: "global"}},
		{name: "invalid producer value", in: IntegrationsStruct{NatsEnable: "yes"}, wantErr: "natsEnable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
