package scenario

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	catalog "github.com/msinclair25/cailab/scenarios"
	"go.yaml.in/yaml/v3"
)

const (
	defaultFileName    = "scenario.yaml"
	embeddedPathPrefix = "embedded:scenarios/"
)

func Load(path string) (Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, fmt.Errorf("read scenario %q: %w", path, err)
	}

	s, err := Decode(data, filepath.Ext(path))
	if err != nil {
		return Scenario{}, fmt.Errorf("decode scenario %q: %w", path, err)
	}
	return s, nil
}

func Decode(data []byte, extension string) (Scenario, error) {
	var s Scenario
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Scenario{}, errors.New("scenario is empty")
	}

	isJSON := strings.EqualFold(extension, ".json") || trimmed[0] == '{'
	if isJSON {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&s); err != nil {
			return Scenario{}, fmt.Errorf("parse JSON: %w", err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return Scenario{}, err
		}
	} else {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&s); err != nil {
			return Scenario{}, fmt.Errorf("parse YAML: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return Scenario{}, errors.New("parse YAML: multiple documents are not supported")
			}
			return Scenario{}, fmt.Errorf("parse YAML trailer: %w", err)
		}
	}

	if err := Validate(s); err != nil {
		return Scenario{}, err
	}
	return s, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("parse JSON: multiple values are not supported")
		}
		return fmt.Errorf("parse JSON trailer: %w", err)
	}
	return nil
}

// Resolve returns a scenario path from either an explicit file or a catalog name.
func Resolve(root, nameOrPath string) (string, error) {
	if info, err := os.Stat(nameOrPath); err == nil && !info.IsDir() {
		return nameOrPath, nil
	}
	if filepath.Base(nameOrPath) != nameOrPath {
		return "", fmt.Errorf("scenario path %q does not exist", nameOrPath)
	}
	path := filepath.Join(root, nameOrPath, defaultFileName)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", fmt.Errorf("scenario %q not found under %q", nameOrPath, root)
	}
	return path, nil
}

// LoadReference loads an explicit scenario file, a scenario from an explicit
// filesystem catalog, or a named scenario from the built-in catalog when root
// is empty. References with explicit path syntax always take precedence.
func LoadReference(root, nameOrPath string) (Scenario, error) {
	if isExplicitScenarioPath(nameOrPath) {
		if info, err := os.Stat(nameOrPath); err != nil || info.IsDir() {
			return Scenario{}, fmt.Errorf("scenario path %q does not exist", nameOrPath)
		}
		return Load(nameOrPath)
	}
	if root != "" {
		resolved, err := Resolve(root, nameOrPath)
		if err != nil {
			return Scenario{}, err
		}
		return Load(resolved)
	}

	embeddedPath := path.Join(nameOrPath, defaultFileName)
	data, err := fs.ReadFile(catalog.Files(), embeddedPath)
	if err != nil {
		return Scenario{}, fmt.Errorf("built-in scenario %q not found: %w", nameOrPath, err)
	}
	definition, err := Decode(data, filepath.Ext(embeddedPath))
	if err != nil {
		return Scenario{}, fmt.Errorf("decode built-in scenario %q: %w", nameOrPath, err)
	}
	return definition, nil
}

func isExplicitScenarioPath(reference string) bool {
	if filepath.Base(reference) != reference {
		return true
	}
	switch strings.ToLower(filepath.Ext(reference)) {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

type Summary struct {
	Name       string
	Version    string
	Title      string
	Path       string
	Objectives int
}

func List(root string) ([]Summary, error) {
	if root == "" {
		return listEmbedded()
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read scenario catalog %q: %w", root, err)
	}

	var summaries []Summary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), defaultFileName)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		s, err := Load(path)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, Summary{
			Name:       s.Metadata.Name,
			Version:    s.Metadata.Version,
			Title:      s.Metadata.Title,
			Path:       path,
			Objectives: len(s.Spec.Objectives),
		})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	return summaries, nil
}

func listEmbedded() ([]Summary, error) {
	files := catalog.Files()
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read built-in scenario catalog: %w", err)
	}

	var summaries []Summary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		embeddedPath := path.Join(entry.Name(), defaultFileName)
		data, err := fs.ReadFile(files, embeddedPath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read built-in scenario %q: %w", entry.Name(), err)
		}
		definition, err := Decode(data, filepath.Ext(embeddedPath))
		if err != nil {
			return nil, fmt.Errorf("decode built-in scenario %q: %w", entry.Name(), err)
		}
		summaries = append(summaries, Summary{
			Name:       definition.Metadata.Name,
			Version:    definition.Metadata.Version,
			Title:      definition.Metadata.Title,
			Path:       embeddedPathPrefix + embeddedPath,
			Objectives: len(definition.Spec.Objectives),
		})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	return summaries, nil
}
