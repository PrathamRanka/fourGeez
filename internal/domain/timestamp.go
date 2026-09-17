package domain

import (
	"encoding/json"
	"time"
)

// Timestamp stores an instant normalized to UTC without a monotonic clock value.
type Timestamp struct {
	value time.Time
}

// NewTimestamp normalizes value to UTC.
func NewTimestamp(value time.Time) Timestamp {
	return Timestamp{value: value.UTC().Round(0)}
}

// ParseTimestamp parses an RFC 3339 timestamp and normalizes it to UTC.
func ParseTimestamp(raw string) (Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return Timestamp{}, NewValidationError("timestamp", "rfc3339", "must be a valid RFC 3339 timestamp")
	}
	return NewTimestamp(parsed), nil
}

// Time returns the normalized time value.
func (timestamp Timestamp) Time() time.Time {
	return timestamp.value
}

// String returns an RFC 3339 timestamp using only required fractional digits.
func (timestamp Timestamp) String() string {
	return timestamp.value.Format(time.RFC3339Nano)
}

// Add returns a timestamp shifted by duration.
func (timestamp Timestamp) Add(duration time.Duration) Timestamp {
	return NewTimestamp(timestamp.value.Add(duration))
}

// Before reports whether timestamp occurs before other.
func (timestamp Timestamp) Before(other Timestamp) bool {
	return timestamp.value.Before(other.value)
}

// MarshalJSON serializes timestamps as RFC 3339 UTC strings.
func (timestamp Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(timestamp.String())
}

// UnmarshalJSON parses an RFC 3339 string and normalizes it to UTC.
func (timestamp *Timestamp) UnmarshalJSON(encoded []byte) error {
	var raw string
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return NewValidationError("timestamp", "rfc3339", "must be a JSON string containing an RFC 3339 timestamp")
	}
	parsed, err := ParseTimestamp(raw)
	if err != nil {
		return err
	}
	*timestamp = parsed
	return nil
}

// Clock provides the current time at a domain boundary.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the process clock and normalizes it to UTC.
type SystemClock struct{}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time {
	return time.Now().UTC().Round(0)
}

// FixedClock is a deterministic clock for tests and controlled workflows.
type FixedClock struct {
	Value time.Time
}

// Now returns the configured value normalized to UTC.
func (clock FixedClock) Now() time.Time {
	return clock.Value.UTC().Round(0)
}
