package verify

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/msinclair25/cailab/internal/graph"
	"github.com/msinclair25/cailab/internal/scenario"
)

type Report struct {
	RunID           string   `json:"runId"`
	Scenario        string   `json:"scenario"`
	ScenarioVersion string   `json:"scenarioVersion"`
	Digest          string   `json:"digest"`
	Passed          bool     `json:"passed"`
	PassedCount     int      `json:"passedCount"`
	FailedCount     int      `json:"failedCount"`
	Results         []Result `json:"results"`
}

type Result struct {
	InvariantID string   `json:"invariantId"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Passed      bool     `json:"passed"`
	Message     string   `json:"message"`
	Evidence    []string `json:"evidence,omitempty"`
}

func Evaluate(runID string, compiled scenario.Compiled) (Report, error) {
	g, err := graph.New(compiled.Nodes, compiled.Edges)
	if err != nil {
		return Report{}, fmt.Errorf("build verification graph: %w", err)
	}

	report := Report{
		RunID: runID, Scenario: compiled.ScenarioName,
		ScenarioVersion: compiled.ScenarioVersion, Digest: compiled.Digest,
		Passed: true, Results: make([]Result, 0, len(compiled.Invariants)),
	}
	for _, invariant := range compiled.Invariants {
		path, exists := g.FindPath(invariant.From, invariant.To)
		result := Result{
			InvariantID: invariant.ID, Severity: invariant.Severity,
			Description: invariant.Description,
		}
		switch invariant.Type {
		case "path_exists":
			result.Passed = exists
			if exists {
				result.Message = "required path exists"
				result.Evidence = pathEvidence(path)
			} else {
				result.Message = "required path does not exist"
			}
		case "path_absent":
			result.Passed = !exists
			if exists {
				result.Message = "prohibited path exists"
				result.Evidence = pathEvidence(path)
			} else {
				result.Message = "prohibited path is absent"
			}
		default:
			return Report{}, fmt.Errorf("invariant %q has unsupported type %q", invariant.ID, invariant.Type)
		}
		if result.Passed {
			report.PassedCount++
		} else {
			report.FailedCount++
			report.Passed = false
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func pathEvidence(path graph.Path) []string {
	if len(path.Nodes) == 0 {
		return nil
	}
	evidence := make([]string, 0, len(path.Edges))
	for i, edge := range path.Edges {
		actions := ""
		if len(edge.Actions) > 0 {
			actions = " [" + strings.Join(edge.Actions, ", ") + "]"
		}
		evidence = append(evidence, fmt.Sprintf("%s --%s%s--> %s", path.Nodes[i], edge.Type, actions, path.Nodes[i+1]))
	}
	if len(evidence) == 0 {
		return []string{path.Nodes[0]}
	}
	return evidence
}

func Markdown(report Report) string {
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# CloudAILab verification: %s\n\n", report.Scenario)
	fmt.Fprintf(&b, "**Status:** %s  \n", status)
	fmt.Fprintf(&b, "**Scenario version:** %s  \n", report.ScenarioVersion)
	fmt.Fprintf(&b, "**Run:** `%s`  \n", report.RunID)
	fmt.Fprintf(&b, "**Results:** %d passed, %d failed\n\n", report.PassedCount, report.FailedCount)
	for _, result := range report.Results {
		mark := "PASS"
		if !result.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "## %s — %s\n\n", mark, result.InvariantID)
		fmt.Fprintf(&b, "%s\n\n", result.Description)
		fmt.Fprintf(&b, "Severity: `%s`  \nResult: %s\n", result.Severity, result.Message)
		if len(result.Evidence) > 0 {
			b.WriteString("\nEvidence:\n\n")
			for _, evidence := range result.Evidence {
				fmt.Fprintf(&b, "- `%s`\n", evidence)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

type junitTestSuite struct {
	XMLName    xml.Name        `xml:"testsuite"`
	Name       string          `xml:"name,attr"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Properties junitProperties `xml:"properties"`
	Cases      []junitTestCase `xml:"testcase"`
}

type junitProperties struct {
	Values []junitProperty `xml:"property"`
}

type junitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

// JUnit projects deterministic invariant results into a timestamp-free JUnit
// testsuite. Each invariant is one testcase; failed invariants are failures.
func JUnit(report Report) ([]byte, error) {
	suite := junitTestSuite{
		Name:  "CloudAILab verification: " + report.Scenario,
		Tests: len(report.Results),
		Properties: junitProperties{Values: []junitProperty{
			{Name: "cloudailab.run_id", Value: report.RunID},
			{Name: "cloudailab.scenario", Value: report.Scenario},
			{Name: "cloudailab.scenario_version", Value: report.ScenarioVersion},
			{Name: "cloudailab.plan_digest", Value: report.Digest},
		}},
		Cases: make([]junitTestCase, 0, len(report.Results)),
	}
	for _, result := range report.Results {
		testCase := junitTestCase{
			Name:      result.InvariantID,
			ClassName: "cloudailab.verify." + report.Scenario,
			SystemOut: strings.Join(result.Evidence, "\n"),
		}
		if !result.Passed {
			suite.Failures++
			testCase.Failure = &junitFailure{
				Message: result.Message,
				Type:    "cloudailab.invariant." + result.Severity,
				Body:    result.Description,
			}
		}
		suite.Cases = append(suite.Cases, testCase)
	}
	encoded, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode JUnit verification report: %w", err)
	}
	return append([]byte(xml.Header), append(encoded, '\n')...), nil
}
