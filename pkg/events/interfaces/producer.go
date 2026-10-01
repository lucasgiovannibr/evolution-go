package producer_interfaces

import "context"

type Producer interface {
	Produce(queueName string, payload []byte, webhookUrl string, userID string) error
	CreateGlobalQueues() error
}

// WebhookStats is the state of the webhook delivery queues (see webhook_producer).
type WebhookStats struct {
	// Destinations is the number of URLs with events pending or in flight.
	Destinations int `json:"destinations"`
	// DegradedDestinations are those whose last event exhausted its retries.
	DegradedDestinations int   `json:"degradedDestinations"`
	Pending              int   `json:"pending"`
	InFlight             int   `json:"inFlight"`
	PendingBytes         int64 `json:"pendingBytes"`
	MaxEvents            int   `json:"maxEventsPerDestination"`
	Workers              int   `json:"workersPerDestination"`
	// Sent, Failed and Dropped count since the process started. Dropped is what
	// overflowed a full queue; Failed exhausted its retries.
	Sent    uint64 `json:"sent"`
	Failed  uint64 `json:"failed"`
	Dropped uint64 `json:"dropped"`
}

// StatsProducer is implemented by producers that can report WebhookStats.
type StatsProducer interface {
	WebhookStats() WebhookStats
}

// Closer is implemented by producers that hold a connection: Close flushes what is
// pending and releases it, within ctx. Called at shutdown.
type Closer interface {
	Close(ctx context.Context) error
}
