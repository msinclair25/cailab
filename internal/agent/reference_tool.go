package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

func ServeReferenceTool(ctx context.Context, input io.Reader, output io.Writer, expectedTool string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	request, err := decodeSingleToolRequest(input)
	if err != nil {
		return fmt.Errorf("read reference tool request: %w", err)
	}
	if request.Tool != expectedTool {
		return fmt.Errorf("expected tool %q, got %q", expectedTool, request.Tool)
	}
	content, _ := json.Marshal(map[string]any{"ok": true, "tool": request.Tool})
	response := ToolExecutionResponse{ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "succeeded", Content: content}
	return writeSingleToolResponse(output, response)
}
