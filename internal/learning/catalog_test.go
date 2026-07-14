package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryLearningCatalog(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	if err := ValidateRepository(root); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeStrictRejectsDuplicateKeys(t *testing.T) {
	var catalog Catalog
	if err := decodeStrict([]byte(`{"apiVersion":"one","apiVersion":"two"}`), &catalog); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("decodeStrict() error = %v, want duplicate-key refusal", err)
	}
}

func TestValidateCatalogRejectsBrokenReferencesAndOrdering(t *testing.T) {
	root := t.TempDir()
	guide := filepath.Join(root, "docs", "guide.md")
	if err := os.MkdirAll(filepath.Dir(guide), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guide, []byte("# guide\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(*Catalog)
		want string
	}{
		{"unknown core", func(catalog *Catalog) { catalog.Lessons[0].CommonCore[0] = "core:missing" }, "unknown common-core"},
		{"duplicate lesson", func(catalog *Catalog) { catalog.Lessons[1].ID = catalog.Lessons[0].ID }, "duplicate lesson"},
		{"unknown prerequisite", func(catalog *Catalog) { catalog.Lessons[1].Prerequisites[0] = "lesson:missing" }, "unknown prerequisite"},
		{"cycle", func(catalog *Catalog) { catalog.Lessons[0].Prerequisites = []string{"lesson:second"} }, "cycle"},
		{"missing guide", func(catalog *Catalog) { catalog.Lessons[0].Binding.Guide = "docs/missing.md" }, "guide"},
		{"missing scenario", func(catalog *Catalog) { catalog.Lessons[0].Binding.Type = "scenario" }, "scenario binding"},
		{"hint order", func(catalog *Catalog) { catalog.Lessons[0].Hints[0].Level = 2 }, "hint levels"},
		{"cleanup mismatch", func(catalog *Catalog) { catalog.Lessons[0].Cleanup.Required = true }, "cleanup"},
		{"path order", func(catalog *Catalog) { catalog.Paths[0].Lessons = []string{"lesson:second", "lesson:first"} }, "before prerequisite"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := testCatalog()
			test.edit(&catalog)
			if err := ValidateCatalog(root, catalog); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateCatalog() error = %v, want %q", err, test.want)
			}
		})
	}
}

func testCatalog() Catalog {
	lesson := func(id string, prerequisites []string) Lesson {
		return Lesson{
			ID: id, Title: id, Track: "common_core", Difficulty: "beginner", DurationMinutes: 10,
			Prerequisites: prerequisites, CommonCore: []string{"core:one"}, SafetyBoundary: "read_only",
			Binding:  Binding{Type: "workflow", ID: "workflow:test", Guide: "docs/guide.md"},
			Outcomes: []string{"outcome"}, MissionLayers: MissionLayers{
				Delivery: "not_applicable", Identity: "covered", RuntimeResource: "not_applicable",
				Data: "not_applicable", EvidenceGovernance: "covered", Remediation: "not_applicable",
			},
			Hints: []Hint{{Level: 1, Text: "hint"}}, Verification: Verification{Type: "cli", Commands: []string{"verify"}},
			Cleanup: Cleanup{Required: false}, ProductionContext: "context", Reflection: []string{"question"},
		}
	}
	return Catalog{
		APIVersion: APIVersion, Kind: Kind,
		CommonCore: []CoreOutcome{{ID: "core:one", Title: "one", Description: "one"}},
		Lessons:    []Lesson{lesson("lesson:first", nil), lesson("lesson:second", []string{"lesson:first"})},
		Paths:      []Path{{ID: "path:test", Title: "test", Audience: "test", Description: "test", Lessons: []string{"lesson:first", "lesson:second"}}},
	}
}
