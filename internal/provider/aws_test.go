package provider

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/scenario"
)

func TestTrustPolicyRoundTripAndExplicitDeny(t *testing.T) {
	t.Parallel()
	candidates := map[string][]string{
		"aws:parent-root":   {"arn:aws:iam::111111111111:root", "111111111111"},
		"aws:acquired-root": {"arn:aws:iam::222222222222:root", "222222222222"},
	}
	document := `{
  "Version":"2012-10-17",
  "Statement":[
    {"Effect":"Allow","Principal":"*","Action":"sts:AssumeRole"},
    {"Effect":"Deny","Principal":{"AWS":"arn:aws:iam::111111111111:root"},"Action":["sts:AssumeRole"]}
  ]
}`
	want := []string{"aws:acquired-root"}
	for _, input := range []string{document, url.QueryEscape(document)} {
		got, err := parseTrustedAWSPrincipals(input, candidates)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("trusted principals = %v, want %v", got, want)
		}
	}
}

func TestPolicyDocumentsUseAWSFieldNames(t *testing.T) {
	t.Parallel()
	trust, err := marshalTrustPolicy(
		[]string{"aws:parent-root", "aws:acquired-root"},
		map[string]string{
			"aws:parent-root":   "arn:aws:iam::111111111111:root",
			"aws:acquired-root": "arn:aws:iam::222222222222:root",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trust, `"Action":"sts:AssumeRole"`) {
		t.Fatalf("trust policy = %s", trust)
	}

	identity, err := marshalIdentityPolicy([]scenario.AWSPolicyStatement{{
		Effect: "Allow", Actions: []string{"s3:GetObject"}, Resources: []string{"arn:aws:s3:::example/*"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"Effect":"Allow"`, `"Action":["s3:GetObject"]`, `"Resource":["arn:aws:s3:::example/*"]`} {
		if !strings.Contains(identity, field) {
			t.Fatalf("identity policy %s does not contain %s", identity, field)
		}
	}
	if strings.Contains(identity, `"actions"`) {
		t.Fatalf("identity policy used scenario field names: %s", identity)
	}
}
