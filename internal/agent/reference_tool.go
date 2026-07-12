package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func ServeReferenceTool(ctx context.Context, input io.Reader, output io.Writer, expectedTool string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), MaxFrameBytes+1)
	if !scanner.Scan() {
		return errors.New("reference tool request is missing")
	}
	frame := append([]byte(nil), scanner.Bytes()...)
	if scanner.Scan() {
		return errors.New("reference tool accepts exactly one request")
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read reference tool request: %w", err)
	}
	var request ToolExecutionRequest
	if err := decodeStrict(frame, &request); err != nil {
		return fmt.Errorf("decode reference tool request: %w", err)
	}
	if err := ValidateToolExecutionRequest(request); err != nil {
		return err
	}
	if request.Tool != expectedTool {
		return fmt.Errorf("expected tool %q, got %q", expectedTool, request.Tool)
	}
	content, _ := json.Marshal(map[string]any{"ok": true, "tool": request.Tool})
	response := ToolExecutionResponse{ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "succeeded", Content: content}
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode reference tool response: %w", err)
	}
	if _, err := output.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write reference tool response: %w", err)
	}
	return nil
}
