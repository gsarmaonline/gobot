package claude

import (
	"testing"

	"github.com/gsarma/gobot/internal/executor"
)

func TestParseStreamLine_TextDelta(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello, world!"}]}}`

	chunks := make(chan executor.StreamChunk, 8)
	var result executor.Result

	parseStreamLine(line, chunks, &result)
	close(chunks)

	var got []executor.StreamChunk
	for c := range chunks {
		got = append(got, c)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
	if got[0].Type != "text" {
		t.Errorf("chunk type = %q, want %q", got[0].Type, "text")
	}
	if got[0].Content != "Hello, world!" {
		t.Errorf("chunk content = %q, want %q", got[0].Content, "Hello, world!")
	}
	if result.Output != "Hello, world!" {
		t.Errorf("result.Output = %q, want %q", result.Output, "Hello, world!")
	}
}

func TestParseStreamLine_ToolUse(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`

	chunks := make(chan executor.StreamChunk, 8)
	var result executor.Result

	parseStreamLine(line, chunks, &result)
	close(chunks)

	var got []executor.StreamChunk
	for c := range chunks {
		got = append(got, c)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
	if got[0].Type != "tool_use" {
		t.Errorf("chunk type = %q, want %q", got[0].Type, "tool_use")
	}
	if got[0].Content != "Bash" {
		t.Errorf("chunk content = %q, want %q", got[0].Content, "Bash")
	}
}

func TestParseStreamLine_ResultSuccess(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"final answer","session_id":"abc-123","is_error":false}`

	chunks := make(chan executor.StreamChunk, 8)
	var result executor.Result

	parseStreamLine(line, chunks, &result)
	close(chunks)

	if result.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "abc-123")
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestParseStreamLine_ResultError(t *testing.T) {
	line := `{"type":"result","subtype":"error_during_execution","result":"something went wrong","is_error":true,"session_id":"xyz"}`

	chunks := make(chan executor.StreamChunk, 8)
	var result executor.Result

	parseStreamLine(line, chunks, &result)
	close(chunks)

	if result.Err == nil {
		t.Fatal("expected error, got nil")
	}
	if result.SessionID != "xyz" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "xyz")
	}

	var got []executor.StreamChunk
	for c := range chunks {
		got = append(got, c)
	}
	if len(got) != 1 || got[0].Type != "error" {
		t.Errorf("expected one error chunk, got %v", got)
	}
}

func TestParseStreamLine_InvalidJSON(t *testing.T) {
	chunks := make(chan executor.StreamChunk, 8)
	var result executor.Result

	// Should not panic or emit chunks.
	parseStreamLine("not valid json", chunks, &result)
	close(chunks)

	var got []executor.StreamChunk
	for c := range chunks {
		got = append(got, c)
	}
	if len(got) != 0 {
		t.Errorf("expected no chunks for invalid JSON, got %d", len(got))
	}
}

func TestParseStreamLine_MultipleContentBlocks(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"part1"},{"type":"tool_use","name":"Read"},{"type":"text","text":"part2"}]}}`

	chunks := make(chan executor.StreamChunk, 8)
	var result executor.Result

	parseStreamLine(line, chunks, &result)
	close(chunks)

	var got []executor.StreamChunk
	for c := range chunks {
		got = append(got, c)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}
	if got[0].Type != "text" || got[0].Content != "part1" {
		t.Errorf("chunk[0] = %+v", got[0])
	}
	if got[1].Type != "tool_use" || got[1].Content != "Read" {
		t.Errorf("chunk[1] = %+v", got[1])
	}
	if got[2].Type != "text" || got[2].Content != "part2" {
		t.Errorf("chunk[2] = %+v", got[2])
	}
	if result.Output != "part1part2" {
		t.Errorf("result.Output = %q, want %q", result.Output, "part1part2")
	}
}
