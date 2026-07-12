package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	ErrToolInputSchema = errors.New("invalid tool input schema")
	ErrToolInput       = errors.New("tool input does not match schema")
)

func ValidateToolInput(schemaData, input json.RawMessage) error {
	schema, err := compileToolInputSchema(schemaData)
	if err != nil {
		return err
	}
	if err := rejectDuplicateJSONKeys(input); err != nil {
		return fmt.Errorf("%w: %v", ErrToolInput, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%w: %v", ErrToolInput, err)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("%w: %v", ErrToolInput, err)
	}
	return nil
}

func compileToolInputSchema(data json.RawMessage) (*jsonschema.Schema, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrToolInputSchema, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrToolInputSchema, err)
	}
	if err := rejectRemoteSchemaReferences(document); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const location = "mem://cloudailab/tool-input.json"
	if err := compiler.AddResource(location, document); err != nil {
		return nil, fmt.Errorf("%w: add schema resource: %v", ErrToolInputSchema, err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("%w: compile schema: %v", ErrToolInputSchema, err)
	}
	return schema, nil
}

func rejectRemoteSchemaReferences(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" || key == "$dynamicRef" {
				reference, ok := child.(string)
				if !ok || !strings.HasPrefix(reference, "#") {
					return fmt.Errorf("%w: %s must be a fragment-local reference", ErrToolInputSchema, key)
				}
			}
			if err := rejectRemoteSchemaReferences(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := rejectRemoteSchemaReferences(child); err != nil {
				return err
			}
		}
	}
	return nil
}
