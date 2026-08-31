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
		t.Fatal()
	}
	if Allow([]ImageMethod{MethodPexels}, MethodNanoBanana) {
		t.Fatal()
	}
}

func TestIsUserAndVIP(t *testing.T) {
	if !MethodPexels.IsUserMethod() || MethodPicsum.IsUserMethod() {
		t.Fatal()
	}
	if !MethodNanoBanana.IsVIPMethod() || MethodPexels.IsVIPMethod() {
		t.Fatal()
	}
}
