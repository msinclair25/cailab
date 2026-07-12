package provider

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/msinclair25/cailab/internal/scenario"
)

func TestFlociIntegration(t *testing.T) {
	if os.Getenv("CAILAB_DOCKER_INTEGRATION") != "1" {
		t.Skip("set CAILAB_DOCKER_INTEGRATION=1 to run Docker integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	definition, err := scenario.Load("../../scenarios/aws-cross-account/scenario.yaml")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := scenario.Compile(definition, definition.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewDockerManager()
	runID := "integration-" + time.Now().UTC().Format("20060102t150405.000000000")
	instances, err := manager.Start(ctx, runID, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if err := manager.Stop(cleanupCtx, runID, instances); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()
	if len(instances) != 1 {
		t.Fatalf("instances = %+v", instances)
	}
	endpoint := instances[0].Endpoint

	initial, err := manager.Snapshot(ctx, instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTrustEdge(initial, "aws:parent-root", "aws:acquisition-reader") {
		t.Fatal("initial snapshot is missing the vulnerable parent trust edge")
	}
	if !hasTrustEdge(initial, "aws:acquired-root", "aws:acquisition-reader") {
		t.Fatal("initial snapshot is missing the legitimate acquired trust edge")
	}

	assumed, err := stsClient(endpoint, "us-east-1", "111111111111", localSecret, "").AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn: aws.String("arn:aws:iam::222222222222:role/AcquisitionDataReader"), RoleSessionName: aws.String("cailab-integration"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if assumed.Credentials == nil {
		t.Fatal("AssumeRole returned no credentials")
	}
	credentials := assumed.Credentials
	object, err := s3ClientWithCredentials(
		endpoint, "us-east-1", aws.ToString(credentials.AccessKeyId), aws.ToString(credentials.SecretAccessKey), aws.ToString(credentials.SessionToken),
	).GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("cailab-acquisition-data"), Key: aws.String("restricted/acquisition-summary.txt"),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(object.Body)
	object.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "SYNTHETIC TRAINING DATA") {
		t.Fatalf("object data = %q", data)
	}

	remediatedPolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::222222222222:root"},"Action":"sts:AssumeRole"}]}`
	if _, err := iamClient(endpoint, "us-east-1", "222222222222").UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName: aws.String("AcquisitionDataReader"), PolicyDocument: aws.String(remediatedPolicy),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := stsClient(endpoint, "us-east-1", "111111111111", localSecret, "").AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn: aws.String("arn:aws:iam::222222222222:role/AcquisitionDataReader"), RoleSessionName: aws.String("cailab-parent-denied"),
	}); err == nil {
		t.Fatal("parent account could still assume the remediated role")
	}
	if _, err := stsClient(endpoint, "us-east-1", "222222222222", localSecret, "").AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn: aws.String("arn:aws:iam::222222222222:role/AcquisitionDataReader"), RoleSessionName: aws.String("cailab-acquired-allowed"),
	}); err != nil {
		t.Fatalf("acquired account lost legitimate role assumption: %v", err)
	}
	remediated, err := manager.Snapshot(ctx, instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	if hasTrustEdge(remediated, "aws:parent-root", "aws:acquisition-reader") {
		t.Fatal("remediated snapshot retained the parent trust edge")
	}
	if !hasTrustEdge(remediated, "aws:acquired-root", "aws:acquisition-reader") {
		t.Fatal("remediated snapshot removed the legitimate acquired trust edge")
	}
}

func hasTrustEdge(compiled scenario.Compiled, from, to string) bool {
	for _, edge := range compiled.Edges {
		if edge.From == from && edge.To == to && edge.Type == "federates_as" {
			return true
		}
	}
	return false
}
