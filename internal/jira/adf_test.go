package jira

import "testing"

func TestMarkdownToADF_SingleParagraph(t *testing.T) {
	doc := markdownToADF("hello world")
	content, ok := doc["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("expected 1 block, got %#v", doc["content"])
	}
	if para, ok := content[0].(map[string]any); !ok || para["type"] != "paragraph" {
		t.Fatalf("expected a paragraph, got %#v", content[0])
	}
}

func TestMarkdownToADF_BlankLineSeparatesParagraphs(t *testing.T) {
	doc := markdownToADF("line one\n\nline two")
	content, ok := doc["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected 2 paragraphs, got %#v", doc["content"])
	}
}

func TestMarkdownToADF_Empty(t *testing.T) {
	doc := markdownToADF("")
	content, ok := doc["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("expected 1 empty paragraph, got %#v", doc["content"])
	}
}

func TestADFRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "soft line breaks", text: "first line\nsecond line"},
		{name: "paragraphs", text: "first paragraph\n\nsecond paragraph"},
		{name: "heading", text: "### Title\n\nbody text"},
		{name: "bullet list", text: "- one\n- two\n- three"},
		{name: "ordered list", text: "1. one\n2. two"},
		{name: "code block", text: "```go\nfmt.Println(\"hi\")\n```"},
		{name: "inline marks", text: "some **bold**, some *italic* and `code`"},
		{name: "link", text: "see [the docs](https://example.com/docs) for more"},
		{name: "blockquote", text: "> quoted line"},
		{name: "rule", text: "before\n\n---\n\nafter"},
		{name: "mixed", text: "# Steps\n\n1. install\n2. run\n\nDone."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adfToMarkdown(markdownToADF(tt.text)); got != tt.text {
				t.Errorf("round trip = %q, want %q", got, tt.text)
			}
		})
	}
}

func TestMarkdownToADF_RejectsUnsafeLinkScheme(t *testing.T) {
	doc := markdownToADF("[click](javascript:alert(1))")
	got := adfToMarkdown(doc)
	if got != "[click](javascript:alert(1))" {
		t.Errorf("adfToMarkdown() = %q, want the literal text", got)
	}

	content, _ := doc["content"].([]any)
	para, _ := content[0].(map[string]any)
	nodes, _ := para["content"].([]any)
	for _, node := range nodes {
		if m, ok := node.(map[string]any); ok {
			if _, hasMarks := m["marks"]; hasMarks {
				t.Fatalf("unsafe link should not produce a link mark: %#v", m)
			}
		}
	}
}

func TestAdfToMarkdown_RejectsUnsafeLinkScheme(t *testing.T) {
	doc := map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type": "paragraph",
			"content": []any{map[string]any{
				"type":  "text",
				"text":  "click",
				"marks": []any{map[string]any{"type": "link", "attrs": map[string]any{"href": "javascript:alert(1)"}}},
			}},
		}},
	}
	if got := adfToMarkdown(doc); got != "click" {
		t.Errorf("adfToMarkdown() = %q, want plain text", got)
	}
}

func TestAdfToMarkdown_Nil(t *testing.T) {
	if got := adfToMarkdown(nil); got != "" {
		t.Errorf("adfToMarkdown(nil) = %q, want empty", got)
	}
}

func TestAdfToMarkdown_RichNodes(t *testing.T) {
	doc := map[string]any{
		"type":    "doc",
		"version": float64(1),
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "mention", "attrs": map[string]any{"text": "@Ada"}},
					map[string]any{"type": "text", "text": " please review"},
				},
			},
			map[string]any{
				"type": "table",
				"content": []any{
					map[string]any{
						"type": "tableRow",
						"content": []any{
							map[string]any{"type": "tableCell", "content": []any{
								map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "a"}}},
							}},
							map[string]any{"type": "tableCell", "content": []any{
								map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "b"}}},
							}},
						},
					},
				},
			},
		},
	}

	want := "@Ada please review\n\na | b"
	if got := adfToMarkdown(doc); got != want {
		t.Errorf("adfToMarkdown() = %q, want %q", got, want)
	}
}
