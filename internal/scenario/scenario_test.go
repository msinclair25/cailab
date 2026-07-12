package scenario

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `apiVersion: cloudailab.dev/v1alpha1
kind: Scenario
metadata:
  name: test-scenario
  version: 0.1.0
  title: Test Scenario
spec:
  seed: 42
  briefing: Test the scenario compiler.
  objectives:
    - id: inspect
      description: Inspect the path.
  tenants:
    - id: tenant-a
      name: Tenant A
      providers: [aws]
  principals:
    - id: principal:a
      tenant: tenant-a
      type: human
      displayName: Principal A
  resources:
    - id: resource:a
      tenant: tenant-a
      type: bucket
      displayName: Resource A
      classification: restricted
  relationships:
    - id: edge:a
      from: principal:a
      to: resource:a
      type: can_access
      actions: [s3:GetObject]
  verification:
    invariants:
      - id: path-visible
        type: path_exists
        from: principal:a
        to: resource:a
        severity: medium
        description: The path is visible.
`

func TestDecodeYAMLStrictly(t *testing.T) {
	t.Parallel()
	s, err := Decode([]byte(validYAML), ".yaml")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if s.Metadata.Name != "test-scenario" {
		t.Fatalf("Metadata.Name = %q, want test-scenario", s.Metadata.Name)
	}

	invalid := strings.Replace(validYAML, "  seed: 42", "  seed: 42\n  unexpected: true", 1)
	_, err = Decode([]byte(invalid), ".yaml")
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("Decode() unknown field error = %v", err)
	}
}

func TestValidateReportsSortedIssues(t *testing.T) {
	t.Parallel()
	s, err := Decode([]byte(validYAML), ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	s.Spec.Principals[0].Tenant = "missing"
	s.Spec.Relationships[0].To = "missing-resource"
	err = Validate(s)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate() error = %T %v, want *ValidationError", err, err)
	}
	if len(validationErr.Issues) != 2 {
		t.Fatalf("issues = %v, want 2", validationErr.Issues)
	}
	if validationErr.Issues[0] > validationErr.Issues[1] {
		t.Fatalf("issues are not sorted: %v", validationErr.Issues)
	}
}

func TestRuntimeImageMustBeDigestPinned(t *testing.T) {
	t.Parallel()
	withRuntime := strings.Replace(validYAML, "  objectives:\n", `  runtimes:
    aws:
      engine: floci
      image: floci/floci:1.5.32@sha256:4f69631e560120d79ad82d2af9f7dda8c6ef7ecbbae0c43ddcffa109c6588a15
      iamEnforcement: true
  objectives:
`, 1)
	s, err := Decode([]byte(withRuntime), ".yaml")
	if err != nil {
		t.Fatalf("Decode() pinned runtime error = %v", err)
	}
	compiled, err := Compile(s, s.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Runtimes.AWS == nil || !compiled.Runtimes.AWS.IAMEnforcement {
		t.Fatalf("compiled runtime = %+v", compiled.Runtimes.AWS)
	}

	unpinned := strings.Replace(withRuntime,
		FlociImage, "floci/floci:latest", 1)
	_, err = Decode([]byte(unpinned), ".yaml")
	if err == nil || !strings.Contains(err.Error(), "supported pinned image") {
		t.Fatalf("Decode() unpinned runtime error = %v", err)
	}
}

func TestCompileIsDeterministic(t *testing.T) {
	t.Parallel()
	s, err := Decode([]byte(validYAML), ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	first, err := Compile(s, 99)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(s, 99)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("digests differ: %q != %q", first.Digest, second.Digest)
	}
	different, err := Compile(s, 100)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest == different.Digest {
		t.Fatal("digest did not change when seed changed")
	}
}

func TestRepositorySchemaAndReferenceScenario(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	schemaData, err := os.ReadFile(filepath.Join(root, "schemas", "scenario", "v1alpha1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schemaDocument any
	if err := json.Unmarshal(schemaData, &schemaDocument); err != nil {
		t.Fatalf("decode repository JSON Schema: %v", err)
	}

	reference, err := Load(filepath.Join(root, "scenarios", "walking-skeleton", "scenario.yaml"))
	if err != nil {
		t.Fatalf("load reference scenario: %v", err)
	}
	compiled, err := Compile(reference, reference.Spec.Seed)
	if err != nil {
		t.Fatalf("compile reference scenario: %v", err)
	}
	if compiled.Digest == "" {
		t.Fatal("reference scenario compiled without a digest")
	}
	microsoft, err := Load(filepath.Join(root, "scenarios", "microsoft-consent", "scenario.yaml"))
	if err != nil {
		t.Fatalf("load Microsoft scenario: %v", err)
	}
	microsoftCompiled, err := Compile(microsoft, microsoft.Spec.Seed)
	if err != nil {
		t.Fatalf("compile Microsoft scenario: %v", err)
	}
	if microsoftCompiled.Providers.Microsoft == nil || len(microsoftCompiled.Providers.Microsoft.OAuth2PermissionGrants) != 2 {
		t.Fatalf("compiled Microsoft provider = %+v", microsoftCompiled.Providers.Microsoft)
	}
}

func TestMicrosoftGrantReferencesDeclaredDirectoryObjects(t *testing.T) {
	t.Parallel()
	definition, err := Load(filepath.Join("..", "..", "scenarios", "microsoft-consent", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	definition.Spec.Providers.Microsoft.OAuth2PermissionGrants[0].ClientID = "99999999-9999-4999-8999-999999999999"
	err = Validate(definition)
	if err == nil || !strings.Contains(err.Error(), "clientId references unknown client service principal") {
		t.Fatalf("Validate() Microsoft reference error = %v", err)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(validYAML), ".yaml")
	f.Add([]byte(`{"apiVersion":"cloudailab.dev/v1alpha1"}`), ".json")
	f.Fuzz(func(t *testing.T, data []byte, extension string) {
		_, _ = Decode(data, extension)
	})
}
