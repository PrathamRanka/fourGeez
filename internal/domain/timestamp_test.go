package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewTimestampNormalizesToUTC(t *testing.T) {
	t.Parallel()

	local := time.Date(2026, time.September, 17, 15, 30, 45, 123456789, time.FixedZone("IST", 5*60*60+30*60))
	timestamp := NewTimestamp(local)

	if timestamp.Time().Location() != time.UTC {
		t.Fatalf("Timestamp location = %v, want UTC", timestamp.Time().Location())
	}
	if timestamp.String() != "2026-09-17T10:00:45.123456789Z" {
		t.Fatalf("Timestamp.String() = %q", timestamp.String())
	}
}

func TestTimestampJSONRoundTrip(t *testing.T) {
	t.Parallel()

	want := NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(encoded) != `"2026-09-17T10:00:00Z"` {
		t.Fatalf("json.Marshal() = %s", encoded)
	}

	var decoded Timestamp
	if err := json.Unmarshal([]byte(`"2026-09-17T15:30:00+05:30"`), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded != want {
		t.Fatalf("decoded = %s, want %s", decoded, want)
	}
}

func TestTimestampRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	var timestamp Timestamp
	if err := json.Unmarshal([]byte(`"not-a-time"`), &timestamp); err == nil {
		t.Fatal("json.Unmarshal() accepted invalid timestamp")
	}
}

func TestFixedClockReturnsNormalizedUTC(t *testing.T) {
	t.Parallel()

	clock := FixedClock{Value: time.Date(2026, time.September, 17, 15, 30, 0, 0, time.FixedZone("IST", 5*60*60+30*60))}
	if got := clock.Now(); got.Location() != time.UTC || got.Hour() != 10 {
		t.Fatalf("FixedClock.Now() = %v, want 10:00 UTC", got)
	}
}
