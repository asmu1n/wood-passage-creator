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

func TestImageResultJSONContract(t *testing.T) {
	image := ImageResult{
		Position: 1, URL: "https://example.com/image.jpg", Method: MethodPexels,
		Keywords: "workbench", SectionTitle: "Introduction", Description: "Cover",
	}
	encoded, err := json.Marshal(image)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"position": "1", "url": `"https://example.com/image.jpg"`, "method": `"PEXELS"`,
		"keywords": `"workbench"`, "sectionTitle": `"Introduction"`, "description": `"Cover"`,
	}
	if len(fields) != len(want) {
		t.Fatalf("unexpected image fields: %s", encoded)
	}
	for name, value := range want {
		if string(fields[name]) != value {
			t.Errorf("%s = %s, want %s", name, fields[name], value)
		}
	}
}
