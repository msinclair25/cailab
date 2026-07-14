package learning

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	APIVersion = "cloudailab.dev/learning/v1alpha1"
	Kind       = "LearningCatalog"
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9._:-]{0,127}$`)

type Catalog struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	CommonCore []CoreOutcome `json:"commonCore"`
	Lessons    []Lesson      `json:"lessons"`
	Paths      []Path        `json:"paths"`
}

type CoreOutcome struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Lesson struct {
	ID                string        `json:"id"`
	Title             string        `json:"title"`
	Track             string        `json:"track"`
	Difficulty        string        `json:"difficulty"`
	DurationMinutes   int           `json:"durationMinutes"`
	Prerequisites     []string      `json:"prerequisites"`
	CommonCore        []string      `json:"commonCore"`
	SafetyBoundary    string        `json:"safetyBoundary"`
	Binding           Binding       `json:"binding"`
	Outcomes          []string      `json:"outcomes"`
	MissionLayers     MissionLayers `json:"missionLayers"`
	Hints             []Hint        `json:"hints"`
	Verification      Verification  `json:"verification"`
	Cleanup           Cleanup       `json:"cleanup"`
	ProductionContext string        `json:"productionContext"`
	Reflection        []string      `json:"reflection"`
}

type Binding struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Guide string `json:"guide"`
}

type MissionLayers struct {
	Delivery           string `json:"delivery"`
	Identity           string `json:"identity"`
	RuntimeResource    string `json:"runtimeResource"`
	Data               string `json:"data"`
	EvidenceGovernance string `json:"evidenceGovernance"`
	Remediation        string `json:"remediation"`
}

type Hint struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type Verification struct {
	Type     string   `json:"type"`
	Commands []string `json:"commands"`
}

type Cleanup struct {
	Required bool     `json:"required"`
	Commands []string `json:"commands"`
}

type Path struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Audience    string   `json:"audience"`
	Description string   `json:"description"`
	Lessons     []string `json:"lessons"`
}

func ValidateRepository(root string) error {
	schemaPath := filepath.Join(root, "schemas", "learning", "v1alpha1", "learning-catalog.json")
	catalogPath := filepath.Join(root, "learning", "catalog.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read learning schema: %w", err)
	}
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("read learning catalog: %w", err)
	}
	if err := validateSchema(schemaData, catalogData); err != nil {
		return err
	}
	var catalog Catalog
	if err := decodeStrict(catalogData, &catalog); err != nil {
		return fmt.Errorf("decode learning catalog: %w", err)
	}
	return ValidateCatalog(root, catalog)
}

func validateSchema(schemaData, catalogData []byte) error {
	var schemaDocument, catalogDocument any
	if err := decodeJSON(schemaData, &schemaDocument); err != nil {
		return fmt.Errorf("decode learning schema: %w", err)
	}
	if err := decodeJSON(catalogData, &catalogDocument); err != nil {
		return fmt.Errorf("decode learning catalog for schema validation: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const location = "mem://cloudailab/learning-catalog.json"
	if err := compiler.AddResource(location, schemaDocument); err != nil {
		return fmt.Errorf("add learning schema: %w", err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return fmt.Errorf("compile learning schema: %w", err)
	}
	if err := schema.Validate(catalogDocument); err != nil {
		return fmt.Errorf("learning catalog does not match schema: %w", err)
	}
	return nil
}

func ValidateCatalog(root string, catalog Catalog) error {
	if catalog.APIVersion != APIVersion || catalog.Kind != Kind {
		return errors.New("learning catalog has an unsupported apiVersion or kind")
	}
	cores := make(map[string]struct{}, len(catalog.CommonCore))
	for _, core := range catalog.CommonCore {
		if err := addID(cores, "common-core outcome", core.ID); err != nil {
			return err
		}
	}
	lessons := make(map[string]Lesson, len(catalog.Lessons))
	for _, lesson := range catalog.Lessons {
		if !identifierPattern.MatchString(lesson.ID) {
			return fmt.Errorf("lesson has invalid id %q", lesson.ID)
		}
		if _, exists := lessons[lesson.ID]; exists {
			return fmt.Errorf("duplicate lesson id %q", lesson.ID)
		}
		lessons[lesson.ID] = lesson
		for _, coreID := range lesson.CommonCore {
			if _, exists := cores[coreID]; !exists {
				return fmt.Errorf("lesson %q references unknown common-core outcome %q", lesson.ID, coreID)
			}
		}
		if err := validateGuide(root, lesson); err != nil {
			return err
		}
		for index, hint := range lesson.Hints {
			if hint.Level != index+1 {
				return fmt.Errorf("lesson %q hint levels must be contiguous and ordered from 1", lesson.ID)
			}
		}
		if lesson.Cleanup.Required != (len(lesson.Cleanup.Commands) > 0) {
			return fmt.Errorf("lesson %q cleanup required flag and commands disagree", lesson.ID)
		}
	}
	if err := validatePrerequisites(lessons); err != nil {
		return err
	}
	pathIDs := make(map[string]struct{}, len(catalog.Paths))
	for _, path := range catalog.Paths {
		if err := addID(pathIDs, "learning path", path.ID); err != nil {
			return err
		}
		seen := make(map[string]int, len(path.Lessons))
		for index, lessonID := range path.Lessons {
			lesson, exists := lessons[lessonID]
			if !exists {
				return fmt.Errorf("learning path %q references unknown lesson %q", path.ID, lessonID)
			}
			seen[lessonID] = index
			for _, prerequisite := range lesson.Prerequisites {
				position, included := seen[prerequisite]
				if !included || position >= index {
					return fmt.Errorf("learning path %q places lesson %q before prerequisite %q", path.ID, lessonID, prerequisite)
				}
			}
		}
	}
	return nil
}

func validateGuide(root string, lesson Lesson) error {
	guide := filepath.Clean(filepath.FromSlash(lesson.Binding.Guide))
	if filepath.IsAbs(guide) || guide == ".." || strings.HasPrefix(guide, ".."+string(filepath.Separator)) {
		return fmt.Errorf("lesson %q guide leaves the repository", lesson.ID)
	}
	info, err := os.Lstat(filepath.Join(root, guide))
	if err != nil {
		return fmt.Errorf("lesson %q guide: %w", lesson.ID, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("lesson %q guide is not a regular file", lesson.ID)
	}
	if lesson.Binding.Type == "scenario" {
		scenarioPath := filepath.Join(root, "scenarios", lesson.Binding.ID, "scenario.yaml")
		scenarioInfo, err := os.Lstat(scenarioPath)
		if err != nil {
			return fmt.Errorf("lesson %q scenario binding: %w", lesson.ID, err)
		}
		if !scenarioInfo.Mode().IsRegular() {
			return fmt.Errorf("lesson %q scenario binding is not a regular file", lesson.ID)
		}
	}
	return nil
}

func validatePrerequisites(lessons map[string]Lesson) error {
	state := make(map[string]uint8, len(lessons))
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("learning prerequisite cycle includes %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, prerequisite := range lessons[id].Prerequisites {
			if _, exists := lessons[prerequisite]; !exists {
				return fmt.Errorf("lesson %q references unknown prerequisite %q", id, prerequisite)
			}
			if err := visit(prerequisite); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range lessons {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func addID(seen map[string]struct{}, kind, id string) error {
	if !identifierPattern.MatchString(id) {
		return fmt.Errorf("%s has invalid id %q", kind, id)
	}
	if _, exists := seen[id]; exists {
		return fmt.Errorf("duplicate %s id %q", kind, id)
	}
	seen[id] = struct{}{}
	return nil
}

func decodeStrict(data []byte, target any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing data")
	}
	return nil
}

func decodeJSON(data []byte, target any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
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
