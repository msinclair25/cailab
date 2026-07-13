package agent

import (
	"fmt"
)

func ValidateTrialStateEvidence(evidence TrialStateEvidence) error {
	var issues []string
	requireEqual(&issues, "apiVersion", evidence.APIVersion, APIVersion)
	requireEqual(&issues, "kind", evidence.Kind, TrialStateEvidenceKind)
	validateID(&issues, "runId", evidence.RunID)
	validateID(&issues, "trialId", evidence.TrialID)
	if evidence.Phase != "before" && evidence.Phase != "after" {
		issues = append(issues, fmt.Sprintf("phase has unsupported value %q", evidence.Phase))
	}
	if evidence.Phase == "after" && evidence.FixtureRestored {
		issues = append(issues, "fixtureRestored is allowed only for before evidence")
	}
	validateTimestamp(&issues, "capturedAt", evidence.CapturedAt)
	validateDigest(&issues, "snapshotDigest", evidence.SnapshotDigest)
	report := evidence.Verification
	if report.RunID != evidence.RunID {
		issues = append(issues, "verification.runId must match runId")
	}
	validateID(&issues, "verification.scenario", report.Scenario)
	validateVersion(&issues, "verification.scenarioVersion", report.ScenarioVersion)
	validateDigest(&issues, "verification.digest", report.Digest)
	if report.PassedCount < 0 || report.FailedCount < 0 || report.PassedCount+report.FailedCount != len(report.Results) {
		issues = append(issues, "verification result counts must match results")
	}
	if report.Passed != (report.FailedCount == 0) {
		issues = append(issues, "verification passed status must match failedCount")
	}
	actualPassed := 0
	seen := make(map[string]struct{}, len(report.Results))
	for index, result := range report.Results {
		prefix := fmt.Sprintf("verification.results[%d]", index)
		validateID(&issues, prefix+".invariantId", result.InvariantID)
		if _, exists := seen[result.InvariantID]; exists {
			issues = append(issues, prefix+".invariantId is duplicated")
		}
		seen[result.InvariantID] = struct{}{}
		if !contains([]string{"low", "medium", "high", "critical"}, result.Severity) {
			issues = append(issues, prefix+".severity is unsupported")
		}
		if result.Passed {
			actualPassed++
		}
		requireSafeText(&issues, prefix+".description", result.Description)
		requireSafeText(&issues, prefix+".message", result.Message)
		for evidenceIndex, value := range result.Evidence {
			requireSafeText(&issues, fmt.Sprintf("%s.evidence[%d]", prefix, evidenceIndex), value)
		}
	}
	if actualPassed != report.PassedCount || len(report.Results)-actualPassed != report.FailedCount {
		issues = append(issues, "verification result statuses must match passedCount and failedCount")
	}
	return validationResult(issues)
}
