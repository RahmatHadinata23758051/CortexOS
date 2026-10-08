package knowledge

import (
	"strings"
	"unicode/utf8"
)

// ChunkMarkdown splits Markdown content deterministically by headings and size.
// Heading context is preserved in each chunk to improve retrieval quality.
func ChunkMarkdown(content string, config IngestionConfig) []Chunk {
	if !utf8.ValidString(content) || strings.TrimSpace(content) == "" {
		return nil
	}
	if config.MaxChunkSize <= 0 {
		config.MaxChunkSize = DefaultIngestionConfig().MaxChunkSize
	}
	if config.ChunkOverlap < 0 {
		config.ChunkOverlap = 0
	}
	if config.ChunkOverlap >= config.MaxChunkSize {
		config.ChunkOverlap = config.MaxChunkSize / 10
	}

	sections := splitMarkdownSections(content)
	var chunks []Chunk
	for _, section := range sections {
		if strings.TrimSpace(section.content) == "" {
			continue
		}
		parts := splitText(section.content, config.MaxChunkSize, config.ChunkOverlap)
		for _, part := range parts {
			text := part.text
			if len(section.headings) > 0 {
				prefix := strings.Join(section.headings, " > ") + "\n\n"
				text = prefix + text
			}
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     text,
				HeadingPath: append([]string(nil), section.headings...),
				StartOffset: section.start + part.start,
				EndOffset:   section.start + part.end,
			})
		}
	}
	return chunks
}

type markdownSection struct {
	headings []string
	content  string
	start    int
}

type textPart struct {
	text       string
	start, end int
}

func splitMarkdownSections(content string) []markdownSection {
	lines := strings.SplitAfter(content, "\n")
	sections := make([]markdownSection, 0)
	headings := make([]string, 0)
	var current strings.Builder
	sectionStart := 0
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if level, title, ok := markdownHeading(trimmed); ok {
			if strings.TrimSpace(current.String()) != "" {
				sections = append(sections, markdownSection{headings: append([]string(nil), headings...), content: current.String(), start: sectionStart})
			}
			current.Reset()
			if level <= len(headings) {
				headings = headings[:level-1]
			}
			for len(headings) < level-1 {
				headings = append(headings, "")
			}
			headings = append(headings, title)
			sectionStart = offset + len(line)
		} else {
			current.WriteString(line)
		}
		offset += len(line)
	}
	if strings.TrimSpace(current.String()) != "" {
		sections = append(sections, markdownSection{headings: append([]string(nil), headings...), content: current.String(), start: sectionStart})
	}
	return sections
}

func markdownHeading(line string) (level int, title string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}
	title = strings.TrimSpace(trimmed[level:])
	return level, title, title != ""
}

func splitText(content string, maxSize, overlap int) []textPart {
	if len(content) <= maxSize {
		return []textPart{{text: content, start: 0, end: len(content)}}
	}
	parts := make([]textPart, 0)
	start := 0
	for start < len(content) {
		end := start + maxSize
		if end >= len(content) {
			end = len(content)
		} else {
			// Prefer paragraph or line boundaries while remaining deterministic.
			boundary := strings.LastIndexAny(content[start:end], "\n. ")
			if boundary > maxSize/2 {
				end = start + boundary + 1
			}
		}
		parts = append(parts, textPart{text: content[start:end], start: start, end: end})
		if end >= len(content) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return parts
}
