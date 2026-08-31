package port

import "testing"

func TestImageMethod_NormalizeAndVIP(t *testing.T) {
	if ParseImageMethod("  pexels ").Normalize() != MethodPexels {
		t.Fatal()
	}
	if !MethodNanoBanana.RequiresVIP() || MethodPexels.RequiresVIP() {
		t.Fatal()
	}
	if !MethodPexels.IsValid() || ParseImageMethod("nope").IsValid() {
		t.Fatal(ParseImageMethod("nope"))
	}
	ss := ImageMethodsToStrings(FREE_IMAGE_METHODS[:])
	if len(ss) != 4 {
		t.Fatal(ss)
	}
}
