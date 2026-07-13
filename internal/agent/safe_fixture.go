package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type SafeFixtureAgentConfig struct {
	ID       string
	Version  string
	Exposure EvaluationActionRef
}

// ServeSafeFixtureAgent is a deterministic fixture-specific control baseline.
// It performs the legitimate exposure read and treats returned content as data;
// it never derives another tool call from that content. It is not a model or a
// claim of general prompt-injection resistance.
func ServeSafeFixtureAgent(ctx context.Context, input io.Reader, output io.Writer, config SafeFixtureAgentConfig) error {
	var issues []string
	validateID(&issues, "id", config.ID)
	validateVersion(&issues, "version", config.Version)
	validateEvaluationActionRef(&issues, "exposure", config.Exposure)
	if err := validationResult(issues); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	decoder := NewDecoder(input)
	start, err := decoder.Next()
	if err != nil {
		return fmt.Errorf("read session.start: %w", err)
	}
	if start.Type != MessageSessionStart {
		return fmt.Errorf("expected %s, got %s", MessageSessionStart, start.Type)
	}
	var startPayload SessionStartPayload
	if err := decodeStrict(start.Payload, &startPayload); err != nil {
		return fmt.Errorf("decode session.start: %w", err)
	}
	if !toolRefNamed(startPayload.Tools, config.Exposure.Tool) {
		return errors.New("safe fixture agent is missing its exposure tool")
	}
	encoder := NewEncoder(output)
	if err := encoder.Write(referenceMessage(MessageAgentReady, "message:ready", AgentReadyPayload{AgentID: config.ID, AgentVersion: config.Version})); err != nil {
		return err
	}
	exposureCall := referenceMessage(MessageToolCall, "call:fixture-exposure", ToolCallPayload{
		Tool: config.Exposure.Tool, Action: config.Exposure.Action, Resource: config.Exposure.Resource, Arguments: json.RawMessage(`{}`),
	})
	if err := encoder.Write(exposureCall); err != nil {
		return err
	}
	exposureResult, err := decoder.Next()
	if err != nil {
		return fmt.Errorf("read exposure result: %w", err)
	}
	if exposureResult.Type != MessageToolResult || exposureResult.CorrelationID != exposureCall.ID {
		return errors.New("safe fixture agent received an invalid exposure result")
	}
	var result ToolResultPayload
	if err := decodeStrict(exposureResult.Payload, &result); err != nil {
		return fmt.Errorf("decode exposure result: %w", err)
	}
	if result.Tool != config.Exposure.Tool {
		return errors.New("safe fixture agent received an exposure result for another tool")
	}
	summary := "safe fixture agent could not read the synthetic untrusted content"
	if result.Status == "succeeded" {
		summary = "safe fixture agent treated retrieved content as untrusted data"
	}
	return encoder.Write(referenceMessage(MessageSessionComplete, "message:complete", SessionCompletePayload{Status: "completed", Summary: summary}))
}
