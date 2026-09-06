package jira

import (
	"net/url"
	"strconv"
	"strings"
)

// markdownToADF converts a Markdown-ish string into an Atlassian Document
// Format (ADF) document, as required by the Jira Cloud v3 API for issue
// descriptions and comment bodies.
//
// The supported subset is: paragraphs (separated by blank lines), ATX
// headings, fenced code blocks, bullet and ordered lists, block quotes,
// horizontal rules, and the inline marks `code`, **strong**, *emphasis* and
// [links](https://example.com). Anything else is carried through as literal
// text, so plain text remains valid input.
func markdownToADF(text string) map[string]any {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": parseBlocks(lines),
	}
}

func parseBlocks(lines []string) []any {
	content := make([]any, 0, len(lines))
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])

		switch {
		case line == "":
			i++

		case strings.HasPrefix(line, "```"):
			node, next := parseCodeBlock(lines, i)
			content = append(content, node)
			i = next

		case isRule(line):
			content = append(content, map[string]any{"type": "rule"})
			i++

		case headingLevel(line) > 0:
			level := headingLevel(line)
			content = append(content, map[string]any{
				"type":    "heading",
				"attrs":   map[string]any{"level": level},
				"content": inlineToADF(strings.TrimSpace(line[level:])),
			})
			i++

		case isBulletItem(line):
			node, next := parseList(lines, i, false)
			content = append(content, node)
			i = next

		case isOrderedItem(line):
			node, next := parseList(lines, i, true)
			content = append(content, node)
			i = next

		case strings.HasPrefix(line, ">"):
			node, next := parseBlockquote(lines, i)
			content = append(content, node)
			i = next

		default:
			node, next := parseParagraph(lines, i)
			content = append(content, node)
			i = next
		}
	}

	if len(content) == 0 {
		content = append(content, map[string]any{"type": "paragraph"})
	}
	return content
}

func parseCodeBlock(lines []string, start int) (map[string]any, int) {
	language := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[start]), "```"))
	i := start + 1
	var body []string
	for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
		body = append(body, lines[i])
		i++
	}
	if i < len(lines) {
		i++ // closing fence
	}

	node := map[string]any{"type": "codeBlock"}
	if language != "" {
		node["attrs"] = map[string]any{"language": language}
	}
	if len(body) > 0 {
		node["content"] = []any{textNode(strings.Join(body, "\n"))}
	}
	return node, i
}

func parseList(lines []string, start int, ordered bool) (map[string]any, int) {
	items := []any{}
	i := start
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		var itemText string
		var ok bool
		if ordered {
			itemText, ok = orderedItemText(line)
		} else if isBulletItem(line) {
			itemText, ok = strings.TrimSpace(line[2:]), true
		}
		if !ok {
			break
		}

		items = append(items, map[string]any{
			"type": "listItem",
			"content": []any{map[string]any{
				"type":    "paragraph",
				"content": inlineToADF(itemText),
			}},
		})
		i++
	}

	listType := "bulletList"
	if ordered {
		listType = "orderedList"
	}
	return map[string]any{"type": listType, "content": items}, i
}

func parseBlockquote(lines []string, start int) (map[string]any, int) {
	i := start
	var quoted []string
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, ">") {
			break
		}
		quoted = append(quoted, strings.TrimPrefix(strings.TrimPrefix(line, ">"), " "))
		i++
	}
	return map[string]any{"type": "blockquote", "content": parseBlocks(quoted)}, i
}

// parseParagraph consumes consecutive plain lines, joining them with hard
// breaks so single newlines survive the round trip.
func parseParagraph(lines []string, start int) (map[string]any, int) {
	i := start
	content := []any{}
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" || startsBlock(line) {
			break
		}
		if len(content) > 0 {
			content = append(content, map[string]any{"type": "hardBreak"})
		}
		content = append(content, inlineToADF(line)...)
		i++
	}

	node := map[string]any{"type": "paragraph"}
	if len(content) > 0 {
		node["content"] = content
	}
	return node, i
}

func startsBlock(line string) bool {
	return strings.HasPrefix(line, "```") ||
		isRule(line) ||
		headingLevel(line) > 0 ||
		isBulletItem(line) ||
		isOrderedItem(line) ||
		strings.HasPrefix(line, ">")
}

func isRule(line string) bool {
	return line == "---" || line == "***" || line == "___"
}

func headingLevel(line string) int {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return 0
	}
	return level
}

func isBulletItem(line string) bool {
	return len(line) > 2 && (line[0] == '-' || line[0] == '*' || line[0] == '+') && line[1] == ' '
}

func isOrderedItem(line string) bool {
	_, ok := orderedItemText(line)
	return ok
}

func orderedItemText(line string) (string, bool) {
	dot := strings.IndexByte(line, '.')
	if dot <= 0 || dot+1 >= len(line) || line[dot+1] != ' ' {
		return "", false
	}
	if _, err := strconv.Atoi(line[:dot]); err != nil {
		return "", false
	}
	return strings.TrimSpace(line[dot+2:]), true
}

func textNode(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func markedTextNode(text string, marks ...map[string]any) map[string]any {
	node := textNode(text)
	list := make([]any, len(marks))
	for i, mark := range marks {
		list[i] = mark
	}
	node["marks"] = list
	return node
}

// inlineToADF converts a single line of inline Markdown into ADF text nodes.
func inlineToADF(s string) []any {
	var nodes []any
	var plain strings.Builder

	flush := func() {
		if plain.Len() > 0 {
			nodes = append(nodes, textNode(plain.String()))
			plain.Reset()
		}
	}

	for i := 0; i < len(s); {
		switch {
		case s[i] == '`':
			if end := strings.IndexByte(s[i+1:], '`'); end > 0 {
				flush()
				nodes = append(nodes, markedTextNode(s[i+1:i+1+end], map[string]any{"type": "code"}))
				i += end + 2
				continue
			}

		case strings.HasPrefix(s[i:], "**"):
			if end := strings.Index(s[i+2:], "**"); end > 0 {
				flush()
				nodes = append(nodes, markedTextNode(s[i+2:i+2+end], map[string]any{"type": "strong"}))
				i += end + 4
				continue
			}

		case s[i] == '*':
			if end := strings.IndexByte(s[i+1:], '*'); end > 0 {
				flush()
				nodes = append(nodes, markedTextNode(s[i+1:i+1+end], map[string]any{"type": "em"}))
				i += end + 2
				continue
			}

		case s[i] == '[':
			if label, href, size, ok := parseInlineLink(s[i:]); ok {
				flush()
				nodes = append(nodes, markedTextNode(label, map[string]any{
					"type":  "link",
					"attrs": map[string]any{"href": href},
				}))
				i += size
				continue
			}
		}

		plain.WriteByte(s[i])
		i++
	}
	flush()

	return nodes
}

// parseInlineLink parses a leading "[label](href)" and reports how many
// bytes it consumed. Only http, https and mailto links are recognised;
// anything else is left as literal text.
func parseInlineLink(s string) (label, href string, size int, ok bool) {
	closeBracket := strings.IndexByte(s, ']')
	if closeBracket <= 0 || closeBracket+1 >= len(s) || s[closeBracket+1] != '(' {
		return "", "", 0, false
	}
	rest := s[closeBracket+2:]
	closeParen := strings.IndexByte(rest, ')')
	if closeParen <= 0 {
		return "", "", 0, false
	}

	label = s[1:closeBracket]
	href = rest[:closeParen]
	if label == "" || !isSafeLinkHref(href) {
		return "", "", 0, false
	}
	return label, href, closeBracket + 2 + closeParen + 1, true
}

func isSafeLinkHref(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	default:
		return false
	}
}

// adfToMarkdown renders an ADF document (as decoded from JSON, i.e. built
// from map[string]any and []any values) as Markdown-ish text. Node types it
// does not know are traversed but not otherwise interpreted.
func adfToMarkdown(doc any) string {
	m, ok := doc.(map[string]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	renderBlocks(&sb, childNodes(m), "")
	return strings.TrimRight(sb.String(), "\n")
}

func childNodes(m map[string]any) []any {
	content, _ := m["content"].([]any)
	return content
}

func nodeAttrs(m map[string]any) map[string]any {
	attrs, _ := m["attrs"].(map[string]any)
	return attrs
}

func attrString(m map[string]any, key string) string {
	s, _ := nodeAttrs(m)[key].(string)
	return s
}

func renderBlocks(sb *strings.Builder, nodes []any, indent string) {
	for i, node := range nodes {
		if i > 0 {
			sb.WriteByte('\n')
		}
		renderBlock(sb, node, indent)
	}
}

func renderBlock(sb *strings.Builder, node any, indent string) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}

	switch nodeType, _ := m["type"].(string); nodeType {
	case "paragraph":
		writeLines(sb, indent, inlineText(m))

	case "heading":
		level := 1
		if l, ok := attrInt(m, "level"); ok && l >= 1 && l <= 6 {
			level = l
		}
		writeLines(sb, indent, strings.Repeat("#", level)+" "+inlineText(m))

	case "codeBlock":
		writeLines(sb, indent, "```"+attrString(m, "language"))
		writeLines(sb, indent, inlineText(m))
		writeLines(sb, indent, "```")

	case "bulletList":
		renderList(sb, m, indent, false)

	case "orderedList":
		renderList(sb, m, indent, true)

	case "blockquote":
		renderBlocks(sb, childNodes(m), indent+"> ")

	case "rule":
		writeLines(sb, indent, "---")

	case "table":
		renderTable(sb, m, indent)

	case "text", "hardBreak", "emoji", "mention", "inlineCard", "date":
		writeLines(sb, indent, inlineNodeText(m))

	default:
		renderBlocks(sb, childNodes(m), indent)
	}
}

func attrInt(m map[string]any, key string) (int, bool) {
	switch value := nodeAttrs(m)[key].(type) {
	case int:
		return value, true
	case float64:
		return int(value), value == float64(int(value))
	default:
		return 0, false
	}
}

func renderList(sb *strings.Builder, m map[string]any, indent string, ordered bool) {
	for i, item := range childNodes(m) {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		marker := "- "
		if ordered {
			marker = strconv.Itoa(i+1) + ". "
		}

		var buf strings.Builder
		renderBlocks(&buf, childNodes(itemMap), "")

		for j, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
			prefix := marker
			if j > 0 {
				prefix = strings.Repeat(" ", len(marker))
			}
			sb.WriteString(indent)
			sb.WriteString(prefix)
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
}

func renderTable(sb *strings.Builder, m map[string]any, indent string) {
	for i, row := range childNodes(m) {
		rowMap, ok := row.(map[string]any)
		if !ok {
			continue
		}

		rowCells := childNodes(rowMap)
		cells := make([]string, 0, len(rowCells))
		for _, cell := range rowCells {
			cellMap, ok := cell.(map[string]any)
			if !ok {
				continue
			}
			var buf strings.Builder
			renderBlocks(&buf, childNodes(cellMap), "")
			cells = append(cells, strings.Join(strings.Fields(buf.String()), " "))
		}

		if i > 0 {
			sb.WriteByte('\n')
		}
		writeLines(sb, indent, strings.Join(cells, " | "))
	}
}

// writeLines writes text as one or more indented lines, each terminated by
// a newline.
func writeLines(sb *strings.Builder, indent, text string) {
	for line := range strings.SplitSeq(text, "\n") {
		sb.WriteString(indent)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
}

func inlineText(m map[string]any) string {
	var sb strings.Builder
	for _, child := range childNodes(m) {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		sb.WriteString(inlineNodeText(childMap))
	}
	return sb.String()
}

func inlineNodeText(m map[string]any) string {
	switch nodeType, _ := m["type"].(string); nodeType {
	case "text":
		text, _ := m["text"].(string)
		return applyMarks(text, m["marks"])
	case "hardBreak":
		return "\n"
	case "emoji":
		if text := attrString(m, "text"); text != "" {
			return text
		}
		return attrString(m, "shortName")
	case "mention":
		return attrString(m, "text")
	case "inlineCard":
		return attrString(m, "url")
	case "date":
		return attrString(m, "timestamp")
	default:
		return inlineText(m)
	}
}

func applyMarks(text string, marks any) string {
	list, ok := marks.([]any)
	if !ok {
		return text
	}
	for _, mark := range list {
		m, ok := mark.(map[string]any)
		if !ok {
			continue
		}
		switch markType, _ := m["type"].(string); markType {
		case "code":
			text = "`" + text + "`"
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "strike":
			text = "~~" + text + "~~"
		case "link":
			if href := attrString(m, "href"); href != text && isSafeLinkHref(href) {
				text = "[" + text + "](" + href + ")"
			}
		}
	}
	return text
}
