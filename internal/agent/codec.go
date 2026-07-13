package agent

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

func DecodeToolManifest(data []byte) (ToolManifest, error) {
	if len(data) > MaxFrameBytes {
		return ToolManifest{}, fmt.Errorf("tool manifest exceeds %d bytes", MaxFrameBytes)
	}
	var manifest ToolManifest
	if err := decodeStrict(data, &manifest); err != nil {
		return ToolManifest{}, fmt.Errorf("decode tool manifest: %w", err)
	}
	if err := ValidateToolManifest(manifest); err != nil {
		return ToolManifest{}, err
	}
	return manifest, nil
}

func DecodeAgentRun(data []byte) (AgentRun, error) {
	if len(data) > MaxFrameBytes {
		return AgentRun{}, fmt.Errorf("agent run exceeds %d bytes", MaxFrameBytes)
	}
	var run AgentRun
	if err := decodeStrict(data, &run); err != nil {
		return AgentRun{}, fmt.Errorf("decode agent run: %w", err)
	}
	if err := ValidateAgentRun(run); err != nil {
		return AgentRun{}, err
	}
	return run, nil
}

func DecodeDecisionEvent(data []byte) (DecisionEvent, error) {
	if len(data) > MaxFrameBytes {
		return DecisionEvent{}, fmt.Errorf("decision event exceeds %d bytes", MaxFrameBytes)
	}
	var event DecisionEvent
	if err := decodeStrict(data, &event); err != nil {
		return DecisionEvent{}, fmt.Errorf("decode decision event: %w", err)
	}
	if err := ValidateDecisionEvent(event); err != nil {
		return DecisionEvent{}, err
	}
	return event, nil
}

func DecodeGovernancePolicy(data []byte) (GovernancePolicy, error) {
	if len(data) > MaxFrameBytes {
		return GovernancePolicy{}, fmt.Errorf("governance policy exceeds %d bytes", MaxFrameBytes)
	}
	var policy GovernancePolicy
	if err := decodeStrict(data, &policy); err != nil {
		return GovernancePolicy{}, fmt.Errorf("decode governance policy: %w", err)
	}
	if err := ValidateGovernancePolicy(policy); err != nil {
		return GovernancePolicy{}, err
	}
	return policy, nil
}

func DecodeToolOutcomeEvent(data []byte) (ToolOutcomeEvent, error) {
	if len(data) > MaxFrameBytes {
		return ToolOutcomeEvent{}, fmt.Errorf("tool outcome event exceeds %d bytes", MaxFrameBytes)
	}
	var event ToolOutcomeEvent
	if err := decodeStrict(data, &event); err != nil {
		return ToolOutcomeEvent{}, fmt.Errorf("decode tool outcome event: %w", err)
	}
	if err := ValidateToolOutcomeEvent(event); err != nil {
		return ToolOutcomeEvent{}, err
	}
	return event, nil
}

func DecodeApprovalResolutionEvent(data []byte) (ApprovalResolutionEvent, error) {
	if len(data) > MaxFrameBytes {
		return ApprovalResolutionEvent{}, fmt.Errorf("approval resolution event exceeds %d bytes", MaxFrameBytes)
	}
	var event ApprovalResolutionEvent
	if err := decodeStrict(data, &event); err != nil {
		return ApprovalResolutionEvent{}, fmt.Errorf("decode approval resolution event: %w", err)
	}
	if err := ValidateApprovalResolutionEvent(event); err != nil {
		return ApprovalResolutionEvent{}, err
	}
	return event, nil
}

func DecodeTrialStateEvidence(data []byte) (TrialStateEvidence, error) {
	if len(data) > MaxFrameBytes {
		return TrialStateEvidence{}, fmt.Errorf("trial state evidence exceeds %d bytes", MaxFrameBytes)
	}
	var evidence TrialStateEvidence
	if err := decodeStrict(data, &evidence); err != nil {
		return TrialStateEvidence{}, fmt.Errorf("decode trial state evidence: %w", err)
	}
	if err := ValidateTrialStateEvidence(evidence); err != nil {
		return TrialStateEvidence{}, err
	}
	return evidence, nil
}

func DigestToolManifest(manifest ToolManifest) (string, error) {
	if err := ValidateToolManifest(manifest); err != nil {
		return "", err
	}
	canonicalSchema, err := CanonicalJSON(manifest.Spec.InputSchema)
	if err != nil {
		return "", fmt.Errorf("canonicalize tool input schema: %w", err)
	}
	copy := manifest
	copy.Spec.InputSchema = canonicalSchema
	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("encode tool manifest: %w", err)
	}
	return digestBytes(data), nil
}

func DigestJSON(data []byte) (string, error) {
	canonical, err := CanonicalJSON(data)
	if err != nil {
		return "", err
	}
	return digestBytes(canonical), nil
}

func CanonicalJSON(data []byte) ([]byte, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type Decoder struct {
	scanner *bufio.Scanner
}

func NewDecoder(reader io.Reader) *Decoder {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), MaxFrameBytes+1)
	return &Decoder{scanner: scanner}
}

func (d *Decoder) Next() (Message, error) {
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return Message{}, fmt.Errorf("read agent protocol frame: %w", err)
		}
		return Message{}, io.EOF
	}
	frame := append([]byte(nil), d.scanner.Bytes()...)
	if len(frame) == 0 {
		return Message{}, errors.New("agent protocol frame must not be empty")
	}
	if len(frame) > MaxFrameBytes {
		return Message{}, fmt.Errorf("agent protocol frame exceeds %d bytes", MaxFrameBytes)
	}
	if !utf8.Valid(frame) {
		return Message{}, errors.New("agent protocol frame must be UTF-8")
	}
	var message Message
	if err := decodeStrict(frame, &message); err != nil {
		return Message{}, fmt.Errorf("decode agent protocol frame: %w", err)
	}
	if err := ValidateMessage(message); err != nil {
		return Message{}, err
	}
	return message, nil
}

type Encoder struct {
	encoder *json.Encoder
}

func NewEncoder(writer io.Writer) *Encoder {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return &Encoder{encoder: encoder}
}

func (e *Encoder) Write(message Message) error {
	if err := ValidateMessage(message); err != nil {
		return err
	}
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode agent protocol frame: %w", err)
	}
	if len(data) > MaxFrameBytes {
		return fmt.Errorf("agent protocol frame exceeds %d bytes", MaxFrameBytes)
	}
	if err := e.encoder.Encode(message); err != nil {
		return fmt.Errorf("write agent protocol frame: %w", err)
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("JSON value is empty")
	}
	if !utf8.Valid(data) {
		return errors.New("JSON value must be UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var parseValue func() error
	parseValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			keys := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("JSON object key is not a string")
				}
				if _, exists := keys[key]; exists {
					return fmt.Errorf("duplicate JSON object key %q", key)
				}
				keys[key] = struct{}{}
				if err := parseValue(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return errors.New("JSON object is not closed")
			}
		case '[':
			for decoder.More() {
				if err := parseValue(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return errors.New("JSON array is not closed")
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
		return nil
	}
	if err := parseValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
