package port

import (
	"encoding/json"
	"testing"
)

func TestImageMethod_UnmarshalJSON(t *testing.T) {
	var m ImageMethod
	if err := json.Unmarshal([]byte(`"  pexels "`), &m); err != nil {
		t.Fatal(err)
	}
	if m != MethodPexels {
		t.Fatal(m)
	}
}

func TestAllow(t *testing.T) {
	if !Allow(nil, MethodNanoBanana) {
		t.Fatal("nil should allow every method")
	}
	if Allow([]ImageMethod{}, MethodPexels) {
		t.Fatal("non-nil empty list should deny every method")
	}
	if Allow([]ImageMethod{MethodPexels}, MethodNanoBanana) {
		t.Fatal("method outside allowlist should be denied")
	}
}
