package observability

import "log/slog"

// Event is a stable low-cardinality operational signal consumed by CloudWatch.
type Event string

const (
	EventFacilitatorFailure      Event = "facilitator_failure"
	EventEvidenceFailure         Event = "evidence_failure"
	EventSellerForwardingFailure Event = "seller_forwarding_failure"
	EventWebhookRetryScheduled   Event = "webhook_retry_scheduled"
	EventWebhookDeadLetter       Event = "webhook_dead_letter"
)

// Record emits no customer input, credentials, payment proof, or wallet material.
func Record(event Event) {
	slog.Error("operational event", "operationalEvent", string(event))
}
