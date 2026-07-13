package provider

import (
	"encoding/json"
	"fmt"
)

func cloneProviderState[T any](value T) (T, error) {
	var result T
	data, err := json.Marshal(value)
	if err != nil {
		return result, fmt.Errorf("encode provider baseline: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("decode provider baseline: %w", err)
	}
	return result, nil
}
