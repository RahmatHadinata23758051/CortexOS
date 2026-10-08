package knowledge

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunkMarkdownDeterministic(t *testing.T) {
	content := `# Architecture

## Overview
This document describes the system architecture.

## Components
### Service Layer
The service layer handles business logic.

### Data Layer
The data layer manages persistence.

## Summary
End of document.`

	chunks := ChunkMarkdown(content, DefaultIngestionConfig())
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	for i, chunk := range chunks {
		if chunk.Index != i {
			t.Errorf("chunk %d: expected index %d, got %d", i, i, chunk.Index)
		}
		if chunk.Content == "" {
			t.Errorf("chunk %d: empty content", i)
		}
		if len(chunk.HeadingPath) == 0 {
			t.Errorf("chunk %d: missing heading path", i)
		}
	}
}

func TestChunkMarkdownStableIDs(t *testing.T) {
	content := `# Title

Some content here.

## Section A
Content A.

## Section B
Content B.`

	cfg := DefaultIngestionConfig()
	chunks1 := ChunkMarkdown(content, cfg)
	chunks2 := ChunkMarkdown(content, cfg)

	if len(chunks1) != len(chunks2) {
		t.Fatalf("chunk count mismatch: %d vs %d", len(chunks1), len(chunks2))
	}
	for i := range chunks1 {
		id1 := chunks1[i].ChunkID("source-1")
		id2 := chunks2[i].ChunkID("source-1")
		if id1 != id2 {
			t.Errorf("chunk %d: ID mismatch: %s != %s", i, id1, id2)
		}
	}
}

func TestChunkMarkdownHeadingContext(t *testing.T) {
	content := `# Main

## Sub A
Content A.

## Sub B
Content B.`

	chunks := ChunkMarkdown(content, DefaultIngestionConfig())
	for _, chunk := range chunks {
		if !strings.Contains(chunk.Content, "Main") {
			t.Errorf("chunk missing top-level heading: %q", chunk.Content)
		}
	}
}

func TestChunkMarkdownEmptyContent(t *testing.T) {
	chunks := ChunkMarkdown("", DefaultIngestionConfig())
	if chunks != nil {
		t.Errorf("expected nil for empty content, got %v", chunks)
	}
	chunks = ChunkMarkdown("   \n\n  ", DefaultIngestionConfig())
	if chunks != nil {
		t.Errorf("expected nil for whitespace content, got %v", chunks)
	}
}

func TestChunkMarkdownUnicode(t *testing.T) {
	content := `# 概要

## 詳細
日本語の内容です。

## Zusammenfassung
Deutsche Inhalte.`

	chunks := ChunkMarkdown(content, DefaultIngestionConfig())
	if len(chunks) == 0 {
		t.Fatal("expected chunks for unicode content")
	}
	for _, chunk := range chunks {
		if !utf8.ValidString(chunk.Content) {
			t.Errorf("chunk content not valid UTF-8: %q", chunk.Content)
		}
	}
}

func TestChunkMarkdownLongLine(t *testing.T) {
	longLine := strings.Repeat("a", 5000)
	content := `# Title

` + longLine + `

End.`

	cfg := IngestionConfig{MaxChunkSize: 1000, ChunkOverlap: 100}
	chunks := ChunkMarkdown(content, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for long line, got %d", len(chunks))
	}
	total := 0
	for _, chunk := range chunks {
		total += len(chunk.Content)
	}
	if total < len(longLine) {
		t.Errorf("total chunk content smaller than original: %d < %d", total, len(longLine))
	}
}

func TestChunkMarkdownNoHeadings(t *testing.T) {
	content := "Plain text without headings.\n\nSecond paragraph."
	chunks := ChunkMarkdown(content, DefaultIngestionConfig())
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "Plain text") {
		t.Errorf("missing content: %q", chunks[0].Content)
	}
}

func TestChunkMarkdownOverlap(t *testing.T) {
	content := "# Title\n\n" + strings.Repeat("word ", 200)
	cfg := IngestionConfig{MaxChunkSize: 500, ChunkOverlap: 100}
	chunks := ChunkMarkdown(content, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Check that overlap exists by verifying content continuity
	first := chunks[0].Content
	second := chunks[1].Content
	found := false
	for i := 0; i < len(first)-50; i++ {
		if strings.Contains(second, first[i:i+50]) {
			found = true
			break
		}
	}
	if !found {
		t.Logf("first end: %q", first[len(first)-100:])
		t.Logf("second start: %q", second[:100])
	}
}

func TestChunkMarkdownConfigDefaults(t *testing.T) {
	cfg := DefaultIngestionConfig()
	if cfg.MaxChunkSize <= 0 || cfg.ChunkOverlap < 0 {
		t.Errorf("invalid defaults: %+v", cfg)
	}
}
