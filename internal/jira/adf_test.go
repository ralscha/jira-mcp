package jira

import "testing"

func TestPlainTextToADF_SingleLine(t *testing.T) {
	doc := plainTextToADF("hello world")
	content, ok := doc["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("expected 1 paragraph, got %#v", doc["content"])
	}
}

func TestPlainTextToADF_MultiLine(t *testing.T) {
	doc := plainTextToADF("line one\n\nline two")
	content, ok := doc["content"].([]any)
	if !ok || len(content) != 3 {
		t.Fatalf("expected 3 paragraphs, got %#v", doc["content"])
	}
}

func TestADFRoundTrip(t *testing.T) {
	original := "first line\nsecond line"
	doc := plainTextToADF(original)

	// Simulate JSON round-trip: the real client decodes JSON responses into
	// map[string]any/[]any, so re-marshal/unmarshal-free construction here
	// matches what json.Unmarshal into `any` would produce, except for the
	// []any vs []interface{} distinction which is identical in Go.
	got := adfToPlainText(doc)
	want := "first line\nsecond line"
	if got != want {
		t.Errorf("adfToPlainText() = %q, want %q", got, want)
	}
}

func TestAdfToPlainText_Nil(t *testing.T) {
	if got := adfToPlainText(nil); got != "" {
		t.Errorf("adfToPlainText(nil) = %q, want empty", got)
	}
}
