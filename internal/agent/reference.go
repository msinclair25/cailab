package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// ReferenceAgentConfig identifies the deterministic baseline on agent.ready.
type ReferenceAgentConfig struct {
	ID      string
	Version string
}

// ServeReferenceAgent runs the deterministic protocol baseline. It performs no
// tool calls and exits after acknowledging a valid session.
func ServeReferenceAgent(ctx context.Context, input io.Reader, output io.Writer, config ReferenceAgentConfig) error {
	var issues []string
	validateID(&issues, "id", config.ID)
	validateVersion(&issues, "version", config.Version)
	if err := validationResult(issues); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	start, err := NewDecoder(input).Next()
	if err != nil {
		return fmt.Errorf("read session.start: %w", err)
	}
	if start.Type != MessageSessionStart {
		return fmt.Errorf("expected %s, got %s", MessageSessionStart, start.Type)
	}
	encoder := NewEncoder(output)
	if err := encoder.Write(referenceMessage(MessageAgentReady, "message:ready", AgentReadyPayload{
		AgentID: config.ID, AgentVersion: config.Version,
	})); err != nil {
		return fmt.Errorf("write agent.ready: %w", err)
	}
	if err := encoder.Write(referenceMessage(MessageSessionComplete, "message:complete", SessionCompletePayload{
		Status: "completed", Summary: "deterministic reference agent completed without tool calls",
	})); err != nil {
		return fmt.Errorf("write session.complete: %w", err)
	}
	return nil
}

func referenceMessage(messageType, id string, payload any) Message {
	data, _ := json.Marshal(payload)
	return Message{ProtocolVersion: ProtocolVersion, ID: id, Type: messageType, Payload: data}
}
