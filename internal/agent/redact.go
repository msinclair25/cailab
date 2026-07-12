package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const RedactedValue = "[REDACTED]"

func RedactJSON(data []byte, pointers []string) ([]byte, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("redact JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("redact JSON: %w", err)
	}
	ordered := append([]string(nil), pointers...)
	seen := make(map[string]struct{}, len(ordered))
	for _, pointer := range ordered {
		if !validJSONPointer(pointer) {
			return nil, fmt.Errorf("redact JSON: invalid JSON Pointer %q", pointer)
		}
		if _, exists := seen[pointer]; exists {
			return nil, fmt.Errorf("redact JSON: duplicate JSON Pointer %q", pointer)
		}
		seen[pointer] = struct{}{}
	}
	sort.Slice(ordered, func(i, j int) bool {
		left, right := strings.Count(ordered[i], "/"), strings.Count(ordered[j], "/")
		return left > right || left == right && ordered[i] < ordered[j]
	})
	for _, pointer := range ordered {
		if pointer == "" {
			document = RedactedValue
			continue
		}
		if err := redactPointer(document, pointer); err != nil {
			return nil, fmt.Errorf("redact JSON pointer %q: %w", pointer, err)
		}
	}
	redacted, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode redacted JSON: %w", err)
	}
	return redacted, nil
}

func redactPointer(document any, pointer string) error {
	parts := strings.Split(pointer[1:], "/")
	for i := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(parts[i], "~1", "/"), "~0", "~")
	}
	current := document
	for i, part := range parts {
		last := i == len(parts)-1
		switch value := current.(type) {
		case map[string]any:
			child, exists := value[part]
			if !exists {
				return errors.New("object member does not exist")
			}
			if last {
				value[part] = RedactedValue
				return nil
			}
			current = child
		case []any:
			if part == "-" || len(part) > 1 && part[0] == '0' {
				return errors.New("array index is invalid")
			}
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return errors.New("array index is out of bounds")
			}
			if last {
				value[index] = RedactedValue
				return nil
			}
			current = value[index]
		default:
			return errors.New("pointer traverses a scalar value")
		}
	}
	return errors.New("pointer did not select a value")
}
