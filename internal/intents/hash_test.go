package intents

import "testing"

func TestHashRequestBodyCanonicalizesJSON(t *testing.T) {
	t.Parallel()

	first, err := HashRequestBody([]byte(`{"size":"M","quantity":1,"options":{"color":"blue","gift":false}}`), "application/json")
	if err != nil {
		t.Fatalf("first HashRequestBody() error = %v", err)
	}
	second, err := HashRequestBody([]byte("{\n  \"options\": {\"gift\": false, \"color\": \"blue\"}, \"quantity\": 1.0, \"size\": \"M\"\n}"), "application/json; charset=utf-8")
	if err != nil {
		t.Fatalf("second HashRequestBody() error = %v", err)
	}

	if first != second {
		t.Fatalf("canonical JSON hashes differ: first=%s second=%s", first, second)
	}
}

func TestHashRequestBodyRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := HashRequestBody([]byte(`{"broken":`), "application/json")
	assertIntentValidationField(t, err, "requestBody")
}

func TestHashRequestBodyRejectsDuplicateJSONKeys(t *testing.T) {
	t.Parallel()

	_, err := HashRequestBody([]byte(`{"quantity":1,"quantity":2}`), "application/json")
	assertIntentValidationField(t, err, "requestBody")
}

func TestHashRequestBodyUsesRawBytesForNonJSON(t *testing.T) {
	t.Parallel()

	first, err := HashRequestBody([]byte("hello"), "text/plain")
	if err != nil {
		t.Fatalf("first HashRequestBody() error = %v", err)
	}
	second, err := HashRequestBody([]byte("hello "), "text/plain")
	if err != nil {
		t.Fatalf("second HashRequestBody() error = %v", err)
	}
	if first == second {
		t.Fatal("different raw request bodies produced the same digest")
	}
}

func TestParseSHA256Digest(t *testing.T) {
	t.Parallel()

	raw := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digest, err := ParseSHA256Digest(raw)
	if err != nil {
		t.Fatalf("ParseSHA256Digest() error = %v", err)
	}
	if digest.String() != raw {
		t.Fatalf("digest = %q, want %q", digest, raw)
	}

	for _, invalid := range []string{"", "ABCDEF", raw[:63], raw + "0"} {
		if _, err := ParseSHA256Digest(invalid); err == nil {
			t.Fatalf("ParseSHA256Digest(%q) succeeded", invalid)
		}
	}
}
