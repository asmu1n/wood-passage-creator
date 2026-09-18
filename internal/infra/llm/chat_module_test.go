package llm

import (
	"encoding/json"
	"testing"
)

type nestedSchemaFixture struct {
	Name string `json:"name"`
}

type schemaFixture struct {
	Items []nestedSchemaFixture `json:"items"`
}

func TestReflectJSONSchema(t *testing.T) {
	outputSchema := reflectJSONSchema(&schemaFixture{})
	raw, err := json.Marshal(outputSchema)
	if err != nil {
		t.Fatal(err)
	}

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["type"] != "object" {
		t.Fatalf("root type=%v, want object", schema["type"])
	}
	if _, ok := schema["$schema"]; ok {
		t.Fatalf("unexpected $schema metadata: %s", raw)
	}
	if _, ok := schema["$id"]; ok {
		t.Fatalf("unexpected $id metadata: %s", raw)
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("root additionalProperties=%v, want false", schema["additionalProperties"])
	}

	required, ok := schema["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "items" {
		t.Fatalf("root required=%v, want [items]", schema["required"])
	}
	properties := schema["properties"].(map[string]any)
	items := properties["items"].(map[string]any)["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Fatalf("nested additionalProperties=%v, want false", items["additionalProperties"])
	}
}
