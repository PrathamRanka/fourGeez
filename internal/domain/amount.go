package domain

import (
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
)

var atomicAmountPattern = regexp.MustCompile(`^[0-9]+$`)

// Amount stores a non-negative payment amount in atomic units.
// The canonical representation contains decimal digits without leading zeros,
// except that zero is represented as "0".
type Amount struct {
	atomicUnits string
}

// ParseAmount validates and canonicalizes an atomic-unit amount.
func ParseAmount(raw string) (Amount, error) {
	if !atomicAmountPattern.MatchString(raw) {
		return Amount{}, NewValidationError("amount", "atomic_units", "must contain decimal digits only")
	}

	canonical := strings.TrimLeft(raw, "0")
	if canonical == "" {
		canonical = "0"
	}
	return Amount{atomicUnits: canonical}, nil
}

// MustParseAmount parses raw and panics when raw is invalid. It is intended for
// constants and test fixtures, not request handling.
func MustParseAmount(raw string) Amount {
	amount, err := ParseAmount(raw)
	if err != nil {
		panic(err)
	}
	return amount
}

// String returns the canonical atomic-unit representation.
func (amount Amount) String() string {
	if amount.atomicUnits == "" {
		return "0"
	}
	return amount.atomicUnits
}

// Compare returns -1, 0, or 1 when amount is less than, equal to, or greater
// than other.
func (amount Amount) Compare(other Amount) int {
	return amount.integer().Cmp(other.integer())
}

// Add returns the sum of amount and other.
func (amount Amount) Add(other Amount) Amount {
	sum := new(big.Int).Add(amount.integer(), other.integer())
	return Amount{atomicUnits: sum.String()}
}

// IsZero reports whether the amount contains no atomic units.
func (amount Amount) IsZero() bool {
	return amount.String() == "0"
}

// MarshalJSON ensures amounts are never serialized as floating-point numbers.
func (amount Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(amount.String())
}

// UnmarshalJSON accepts only a JSON string containing atomic units.
func (amount *Amount) UnmarshalJSON(encoded []byte) error {
	var raw string
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return NewValidationError("amount", "atomic_units", "must be a JSON string containing decimal digits only")
	}
	parsed, err := ParseAmount(raw)
	if err != nil {
		return err
	}
	*amount = parsed
	return nil
}

func (amount Amount) integer() *big.Int {
	integer, ok := new(big.Int).SetString(amount.String(), 10)
	if !ok {
		panic("domain.Amount contains an invalid internal value")
	}
	return integer
}
