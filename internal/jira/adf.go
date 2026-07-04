package jira

import "strings"

// plainTextToADF converts plain text into a minimal Atlassian Document
// Format (ADF) document, as required by the Jira Cloud v3 API for issue
// descriptions and comment bodies. Each line of input becomes its own
// paragraph node; blank lines produce empty paragraphs.
//
// This only supports plain text; rich formatting (mentions, tables, links,
// etc.) is not preserved.
func plainTextToADF(text string) map[string]any {
	lines := strings.Split(text, "\n")
	content := make([]any, 0, len(lines))
	for _, line := range lines {
		para := map[string]any{"type": "paragraph"}
		if line != "" {
			para["content"] = []any{
				map[string]any{"type": "text", "text": line},
			}
		}
		content = append(content, para)
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

// adfToPlainText extracts a best-effort plain-text rendering of an ADF
// document (as decoded from JSON, i.e. built from map[string]any and
// []any values). It is intended for read-only display purposes; unknown
// node types are traversed but not otherwise interpreted.
func adfToPlainText(doc any) string {
	var sb strings.Builder
	writeADFNode(&sb, doc)
	return strings.TrimRight(sb.String(), "\n")
}

func writeADFNode(sb *strings.Builder, node any) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}
	nodeType, _ := m["type"].(string)

	switch nodeType {
	case "text":
		if text, ok := m["text"].(string); ok {
			sb.WriteString(text)
		}
	case "hardBreak":
		sb.WriteString("\n")
	}

	if content, ok := m["content"].([]any); ok {
		for _, child := range content {
			writeADFNode(sb, child)
		}
	}

	switch nodeType {
	case "paragraph", "heading", "blockquote", "codeBlock", "listItem":
		sb.WriteString("\n")
	}
}
