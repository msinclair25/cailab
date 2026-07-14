package verify

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/scenario"
)

func TestEvaluatePathInvariants(t *testing.T) {
	t.Parallel()
	compiled := scenario.Compiled{
		ScenarioName: "test", ScenarioVersion: "1", Digest: strings.Repeat("a", 64),
		Nodes: []scenario.Node{{ID: "actor"}, {ID: "resource"}, {ID: "isolated"}},
		Edges: []scenario.Relationship{{ID: "access", From: "actor", To: "resource", Type: "can_access"}},
		Invariants: []scenario.Invariant{
			{ID: "exists", Type: "path_exists", From: "actor", To: "resource", Severity: "medium", Description: "exists"},
			{ID: "absent", Type: "path_absent", From: "actor", To: "isolated", Severity: "high", Description: "absent"},
		},
	}
	report, err := Evaluate("run-1", compiled)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.PassedCount != 2 || report.FailedCount != 0 {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(Markdown(report), "2 passed, 0 failed") {
		t.Fatalf("Markdown() missing summary:\n%s", Markdown(report))
	}
}

func TestEvaluateReportsProhibitedPath(t *testing.T) {
	t.Parallel()
	compiled := scenario.Compiled{
		ScenarioName: "test", ScenarioVersion: "1", Digest: strings.Repeat("b", 64),
		Nodes: []scenario.Node{{ID: "actor"}, {ID: "resource"}},
		Edges: []scenario.Relationship{{ID: "access", From: "actor", To: "resource", Type: "can_access"}},
		Invariants: []scenario.Invariant{{
			ID: "blocked", Type: "path_absent", From: "actor", To: "resource",
			Severity: "critical", Description: "must be blocked",
		}},
	}
	report, err := Evaluate("run-2", compiled)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || report.FailedCount != 1 {
		t.Fatalf("report = %+v", report)
	}
	if got := report.Results[0].Evidence; len(got) != 1 || !strings.Contains(got[0], "can_access") {
		t.Fatalf("evidence = %v", got)
	}
}

func TestJUnitIsDeterministicAndEscapesEvidence(t *testing.T) {
	t.Parallel()
	report := Report{
		RunID: "run:<one>", Scenario: "test&scenario", ScenarioVersion: "1.0.0", Digest: strings.Repeat("c", 64),
		Passed: false, PassedCount: 1, FailedCount: 1,
		Results: []Result{
			{InvariantID: "approved", Severity: "high", Passed: true, Message: "required path exists", Evidence: []string{"actor --can_access--> resource&one"}},
			{InvariantID: "blocked", Severity: "critical", Description: "contractor <must> be blocked", Passed: false, Message: "prohibited path exists"},
		},
	}
	first, err := JUnit(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := JUnit(report)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("JUnit output changed for identical input")
	}
	var document struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(first, &document); err != nil {
		t.Fatalf("JUnit output is invalid XML: %v\n%s", err, first)
	}
	if document.XMLName.Local != "testsuite" {
		t.Fatalf("JUnit root = %q, want testsuite", document.XMLName.Local)
	}
	output := string(first)
	for _, want := range []string{`tests="2"`, `failures="1"`, `name="blocked"`, `type="cloudailab.invariant.critical"`, `resource&amp;one`, `contractor &lt;must&gt; be blocked`} {
		if !strings.Contains(output, want) {
			t.Fatalf("JUnit output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "timestamp=") || strings.Contains(output, "time=") {
		t.Fatalf("JUnit output contains nondeterministic time metadata:\n%s", output)
	}
}
