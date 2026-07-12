package verify

import (
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
