package domain

import (
	"context"
	"testing"
)

func TestParseIdempotencyKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    IdempotencyKey
		wantErr bool
	}{
		{name: "valid", raw: "purchase-001", want: IdempotencyKey("purchase-001")},
		{name: "minimum length", raw: "12345678", want: IdempotencyKey("12345678")},
		{name: "too short", raw: "short", wantErr: true},
		{name: "too long", raw: string(make([]byte, 129)), wantErr: true},
		{name: "surrounding spaces", raw: " purchase-001 ", wantErr: true},
		{name: "control character", raw: "purchase\n001", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			key, err := ParseIdempotencyKey(test.raw)
			if test.wantErr {
				assertValidationError(t, err, "idempotencyKey", "format")
				return
			}
			if err != nil {
				t.Fatalf("ParseIdempotencyKey() error = %v", err)
			}
			if key != test.want {
				t.Fatalf("ParseIdempotencyKey() = %q, want %q", key, test.want)
			}
		})
	}
}

func TestIdempotencyStoreContractCanBeImplemented(t *testing.T) {
	t.Parallel()

	var store IdempotencyStore = idempotencyStoreStub{}
	if _, _, err := store.Load(context.Background(), "seller:create", IdempotencyKey("purchase-001")); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

type idempotencyStoreStub struct{}

func (idempotencyStoreStub) Load(context.Context, string, IdempotencyKey) (IdempotencyRecord, bool, error) {
	return IdempotencyRecord{}, false, nil
}

func (idempotencyStoreStub) SaveIfAbsent(context.Context, IdempotencyRecord) (bool, error) {
	return true, nil
}
