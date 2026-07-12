package scenario

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const defaultFileName = "scenario.yaml"

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

type Summary struct {
	Name       string
	Version    string
	Title      string
	Path       string
	Objectives int
}

func List(root string) ([]Summary, error) {
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
