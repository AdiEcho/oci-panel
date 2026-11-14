package services

import "testing"

func TestDedupeShapes(t *testing.T) {
	shapes := []ShapeInfo{
		{Shape: "VM.Standard.A1.Flex", FreeTier: true},
		{Shape: "VM.Standard.E2.1.Micro"},
		{Shape: "VM.Standard.A1.Flex", FreeTier: true},
		{Shape: ""},
		{Shape: "BM.Standard.A1.160"},
	}
	result := dedupeShapes(shapes)
	if len(result) != 3 {
		t.Fatalf("dedupeShapes len = %d, want 3: %+v", len(result), result)
	}
	if result[0].Shape != "VM.Standard.A1.Flex" || result[2].Shape != "BM.Standard.A1.160" {
		t.Fatalf("dedupeShapes order = %+v", result)
	}
}
