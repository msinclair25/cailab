package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GoogleDriveReadToolConfig struct {
	Tool     string
	Action   string
	Resource string
	Endpoint string
	FileID   string
	Token    string
}

// ServeGoogleDriveReadTool is a code-owned fixture adapter for one configured
// Google Drive media resource. It accepts no model-selected URL or file ID.
func ServeGoogleDriveReadTool(ctx context.Context, input io.Reader, output io.Writer, config GoogleDriveReadToolConfig) error {
	parsed, err := url.Parse(config.Endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("Google Drive fixture endpoint must be an exact IPv4 loopback HTTP origin")
	}
	var issues []string
	validateID(&issues, "tool", config.Tool)
	requireSafeText(&issues, "action", config.Action)
	requireSafeText(&issues, "fileId", config.FileID)
	validateID(&issues, "resource", config.Resource)
	if config.Token == "" {
		issues = append(issues, "token must not be empty")
	}
	if err := validationResult(issues); err != nil {
		return err
	}
	request, err := decodeSingleToolRequest(input)
	if err != nil {
		return fmt.Errorf("read Google Drive tool request: %w", err)
	}
	if request.Tool != config.Tool || request.Action != config.Action || request.Resource.ID != config.Resource {
		return errors.New("Google Drive tool request does not match its fixed fixture target")
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(config.Endpoint, "/")+"/drive/v3/files/"+url.PathEscape(config.FileID)+"?alt=media", nil)
	if err != nil {
		return fmt.Errorf("build Google Drive fixture request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+config.Token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(httpRequest)
	if err != nil {
		return writeSingleToolResponse(output, ToolExecutionResponse{
			ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "failed", ErrorCode: "tool:provider_unavailable",
		})
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, MaxFrameBytes))
		return writeSingleToolResponse(output, ToolExecutionResponse{
			ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "failed", ErrorCode: "tool:provider_rejected",
		})
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxFrameBytes+1))
	if err != nil {
		return fmt.Errorf("read Google Drive fixture content: %w", err)
	}
	if len(data) > MaxFrameBytes {
		return errors.New("Google Drive fixture content exceeds the tool frame limit")
	}
	content, err := json.Marshal(map[string]string{"content": string(data)})
	if err != nil {
		return fmt.Errorf("encode Google Drive fixture content: %w", err)
	}
	return writeSingleToolResponse(output, ToolExecutionResponse{
		ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "succeeded", Content: content,
	})
}

func decodeSingleToolRequest(input io.Reader) (ToolExecutionRequest, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), MaxFrameBytes+1)
	if !scanner.Scan() {
		return ToolExecutionRequest{}, errors.New("tool request is missing")
	}
	frame := append([]byte(nil), scanner.Bytes()...)
	if scanner.Scan() {
		return ToolExecutionRequest{}, errors.New("tool accepts exactly one request")
	}
	if err := scanner.Err(); err != nil {
		return ToolExecutionRequest{}, err
	}
	var request ToolExecutionRequest
	if err := decodeStrict(frame, &request); err != nil {
		return ToolExecutionRequest{}, fmt.Errorf("decode tool request: %w", err)
	}
	if err := ValidateToolExecutionRequest(request); err != nil {
		return ToolExecutionRequest{}, err
	}
	return request, nil
}

func writeSingleToolResponse(output io.Writer, response ToolExecutionResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode tool response: %w", err)
	}
	if _, err := output.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write tool response: %w", err)
	}
	return nil
}
