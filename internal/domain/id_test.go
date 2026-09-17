package domain

import (
	"bytes"
	"testing"
	"time"
)

func TestParseID(t *testing.T) {
	t.Parallel()

	const validULID = "01K5D09YJ0C0M7RJM4FWQ0K9H7"

	tests := []struct {
		name      string
		raw       string
		prefix    IDPrefix
		want      ID
		wantErr   bool
		wantField string
		wantRule  string
	}{
		{name: "seller ID", raw: "sel_" + validULID, prefix: SellerIDPrefix, want: ID("sel_" + validULID)},
		{name: "credential ID", raw: "key_" + validULID, prefix: CredentialIDPrefix, want: ID("key_" + validULID)},
		{name: "wrong prefix", raw: "txn_" + validULID, prefix: SellerIDPrefix, wantErr: true, wantField: "id", wantRule: "prefix"},
		{name: "malformed ULID", raw: "sel_not-a-ulid", prefix: SellerIDPrefix, wantErr: true, wantField: "id", wantRule: "format"},
		{name: "empty", raw: "", prefix: SellerIDPrefix, wantErr: true, wantField: "id", wantRule: "required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			identifier, err := ParseID(test.raw, test.prefix)
			if test.wantErr {
				assertValidationError(t, err, test.wantField, test.wantRule)
				return
			}
			if err != nil {
				t.Fatalf("ParseID() error = %v", err)
			}
			if identifier != test.want {
				t.Fatalf("ParseID() = %q, want %q", identifier, test.want)
			}
		})
	}
}

func TestULIDGeneratorCreatesPrefixedSortableIDs(t *testing.T) {
	t.Parallel()

	clock := FixedClock{Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.FixedZone("IST", 5*60*60+30*60))}
	entropy := bytes.NewReader(bytes.Repeat([]byte{0x2a}, 64))
	generator := NewULIDGenerator(clock, entropy)

	first, err := generator.New(TransactionIDPrefix)
	if err != nil {
		t.Fatalf("first New() error = %v", err)
	}
	second, err := generator.New(TransactionIDPrefix)
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}

	if first.Prefix() != TransactionIDPrefix {
		t.Fatalf("first.Prefix() = %q, want %q", first.Prefix(), TransactionIDPrefix)
	}
	if first.String() >= second.String() {
		t.Fatalf("generated IDs are not sortable: first=%q second=%q", first, second)
	}
	if _, err := ParseID(first.String(), TransactionIDPrefix); err != nil {
		t.Fatalf("generated ID failed parsing: %v", err)
	}
}

func TestULIDGeneratorRejectsUnknownPrefix(t *testing.T) {
	t.Parallel()

	generator := NewULIDGenerator(FixedClock{Value: time.Now()}, bytes.NewReader(bytes.Repeat([]byte{0x01}, 32)))
	_, err := generator.New(IDPrefix("unknown_"))
	assertValidationError(t, err, "prefix", "unsupported")
}
