package domain

import (
	"encoding/json"
	"testing"
)

func TestParseAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "zero", raw: "0", want: "0"},
		{name: "canonicalizes leading zeros", raw: "00035000000", want: "35000000"},
		{name: "arbitrary precision", raw: "999999999999999999999999999999999999", want: "999999999999999999999999999999999999"},
		{name: "empty", raw: "", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "decimal", raw: "35.00", wantErr: true},
		{name: "leading plus", raw: "+35", wantErr: true},
		{name: "spaces", raw: " 35 ", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			amount, err := ParseAmount(test.raw)
			if test.wantErr {
				assertValidationError(t, err, "amount", "atomic_units")
				return
			}
			if err != nil {
				t.Fatalf("ParseAmount() error = %v", err)
			}
			if amount.String() != test.want {
				t.Fatalf("ParseAmount() = %q, want %q", amount, test.want)
			}
		})
	}
}

func TestAmountComparisonAndAddition(t *testing.T) {
	t.Parallel()

	thirtyFive := MustParseAmount("35000000")
	fifty := MustParseAmount("50000000")

	if thirtyFive.Compare(fifty) >= 0 {
		t.Fatal("expected 35000000 to be less than 50000000")
	}
	if fifty.Compare(thirtyFive) <= 0 {
		t.Fatal("expected 50000000 to be greater than 35000000")
	}
	if sum := thirtyFive.Add(MustParseAmount("15000000")); sum != fifty {
		t.Fatalf("Add() = %s, want %s", sum, fifty)
	}
}

func TestAmountJSONUsesString(t *testing.T) {
	t.Parallel()

	amount := MustParseAmount("35000000")
	encoded, err := json.Marshal(amount)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(encoded) != `"35000000"` {
		t.Fatalf("json.Marshal() = %s, want quoted atomic amount", encoded)
	}

	var decoded Amount
	if err := json.Unmarshal([]byte(`"00035000000"`), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded != amount {
		t.Fatalf("decoded amount = %s, want %s", decoded, amount)
	}

	if err := json.Unmarshal([]byte(`35`), &decoded); err == nil {
		t.Fatal("json.Unmarshal() accepted a numeric money value")
	}
}
