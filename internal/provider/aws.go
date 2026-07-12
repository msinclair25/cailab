package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/msinclair25/cailab/internal/scenario"
)

const localSecret = "cailab-local-only"

func hydrateAWS(ctx context.Context, endpoint string, topology *scenario.AWSProvider) error {
	if topology == nil {
		return nil
	}
	principalARNs := awsPrincipalARNs(topology)
	for _, role := range topology.Roles {
		client := iamClient(endpoint, topology.Region, role.Account)
		trustDocument, err := marshalTrustPolicy(role.Trust, principalARNs)
		if err != nil {
			return fmt.Errorf("build trust policy for role %q: %w", role.Name, err)
		}
		if _, err := client.CreateRole(ctx, &iam.CreateRoleInput{
			RoleName:                 aws.String(role.Name),
			AssumeRolePolicyDocument: aws.String(trustDocument),
		}); err != nil {
			return fmt.Errorf("create role %s/%s: %w", role.Account, role.Name, err)
		}
		for _, policy := range role.Policies {
			document, err := marshalIdentityPolicy(policy.Statements)
			if err != nil {
				return fmt.Errorf("build role policy %q: %w", policy.Name, err)
			}
			if _, err := client.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
				RoleName: aws.String(role.Name), PolicyName: aws.String(policy.Name), PolicyDocument: aws.String(document),
			}); err != nil {
				return fmt.Errorf("put policy %s on role %s/%s: %w", policy.Name, role.Account, role.Name, err)
			}
		}
	}
	for _, bucket := range topology.Buckets {
		client := s3Client(endpoint, topology.Region, bucket.Account)
		input := &s3.CreateBucketInput{Bucket: aws.String(bucket.Name)}
		if topology.Region != "us-east-1" {
			input.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(topology.Region),
			}
		}
		if _, err := client.CreateBucket(ctx, input); err != nil {
			return fmt.Errorf("create bucket %s/%s: %w", bucket.Account, bucket.Name, err)
		}
		for _, object := range bucket.Objects {
			if _, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket.Name), Key: aws.String(object.Key), Body: strings.NewReader(object.Data),
			}); err != nil {
				return fmt.Errorf("put object %s/%s/%s: %w", bucket.Account, bucket.Name, object.Key, err)
			}
		}
	}
	return nil
}

func snapshotAWS(ctx context.Context, endpoint string, compiled scenario.Compiled) (scenario.Compiled, error) {
	topology := compiled.Providers.AWS
	if topology == nil {
		return compiled, nil
	}
	principalAliases := make(map[string][]string, len(topology.Accounts))
	for _, account := range topology.Accounts {
		principalAliases[account.Principal] = []string{"arn:aws:iam::" + account.ID + ":root", account.ID}
	}
	roleNodes := make(map[string]struct{}, len(topology.Roles))
	currentTrust := make(map[string][]string, len(topology.Roles))
	for _, role := range topology.Roles {
		roleNodes[role.Node] = struct{}{}
		output, err := iamClient(endpoint, topology.Region, role.Account).GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(role.Name)})
		if err != nil {
			return scenario.Compiled{}, fmt.Errorf("get role %s/%s: %w", role.Account, role.Name, err)
		}
		if output.Role == nil || output.Role.AssumeRolePolicyDocument == nil {
			return scenario.Compiled{}, fmt.Errorf("role %s/%s has no trust policy", role.Account, role.Name)
		}
		principals, err := parseTrustedAWSPrincipals(*output.Role.AssumeRolePolicyDocument, principalAliases)
		if err != nil {
			return scenario.Compiled{}, fmt.Errorf("parse role %s/%s trust policy: %w", role.Account, role.Name, err)
		}
		currentTrust[role.Node] = append(currentTrust[role.Node], principals...)
		sort.Strings(currentTrust[role.Node])
	}

	originalEdges := make(map[string]scenario.Relationship)
	edges := make([]scenario.Relationship, 0, len(compiled.Edges))
	for _, edge := range compiled.Edges {
		if strings.HasPrefix(edge.ID, "aws-web-trust:") {
			continue
		}
		if _, isRole := roleNodes[edge.To]; isRole && edge.Type == "federates_as" {
			originalEdges[edge.From+"\x00"+edge.To] = edge
			continue
		}
		edges = append(edges, edge)
	}
	for roleNode, principals := range currentTrust {
		for _, principal := range principals {
			key := principal + "\x00" + roleNode
			edge, ok := originalEdges[key]
			if !ok {
				edge = scenario.Relationship{
					ID: trustEdgeID(principal, roleNode), From: principal, To: roleNode,
					Type: "federates_as", Actions: []string{"sts:AssumeRole"},
				}
			}
			edges = append(edges, edge)
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	compiled.Edges = edges
	return normalizeAWSWebIdentityTrust(compiled), nil
}

func normalizeAWSWebIdentityTrust(compiled scenario.Compiled) scenario.Compiled {
	if compiled.Providers.AWS == nil {
		return compiled
	}
	edges := make([]scenario.Relationship, 0, len(compiled.Edges)+len(compiled.Providers.AWS.Roles))
	for _, edge := range compiled.Edges {
		if !strings.HasPrefix(edge.ID, "aws-web-trust:") {
			edges = append(edges, edge)
		}
	}
	for _, role := range compiled.Providers.AWS.Roles {
		if role.WebIdentity == nil {
			continue
		}
		edges = append(edges, scenario.Relationship{
			ID:   webTrustEdgeID(role.WebIdentity.AudienceNode, role.Node, role.WebIdentity.Audience),
			From: role.WebIdentity.AudienceNode, To: role.Node, Type: "federates_as",
			Actions: []string{"sts:AssumeRoleWithWebIdentity"},
		})
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	compiled.Edges = edges
	return compiled
}

func iamClient(endpoint, region, accountID string) *iam.Client {
	config := awsClientConfig(region, accountID, localSecret, "")
	return iam.NewFromConfig(config, func(options *iam.Options) {
		options.BaseEndpoint = aws.String(endpoint)
	})
}

func s3Client(endpoint, region, accountID string) *s3.Client {
	return s3ClientWithCredentials(endpoint, region, accountID, localSecret, "")
}

func s3ClientWithCredentials(endpoint, region, accessKey, secretKey, sessionToken string) *s3.Client {
	config := awsClientConfig(region, accessKey, secretKey, sessionToken)
	return s3.NewFromConfig(config, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
}

func stsClient(endpoint, region, accessKey, secretKey, sessionToken string) *sts.Client {
	config := awsClientConfig(region, accessKey, secretKey, sessionToken)
	return sts.NewFromConfig(config, func(options *sts.Options) {
		options.BaseEndpoint = aws.String(endpoint)
	})
}

func awsClientConfig(region, accessKey, secretKey, sessionToken string) aws.Config {
	return aws.Config{
		Region:      region,
		HTTPClient:  &http.Client{Timeout: 5 * time.Second},
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)),
	}
}

func awsPrincipalARNs(topology *scenario.AWSProvider) map[string]string {
	arns := make(map[string]string, len(topology.Accounts))
	for _, account := range topology.Accounts {
		arns[account.Principal] = "arn:aws:iam::" + account.ID + ":root"
	}
	return arns
}

func marshalTrustPolicy(trusted []string, principalARNs map[string]string) (string, error) {
	arns := make([]string, 0, len(trusted))
	for _, principal := range trusted {
		arn, ok := principalARNs[principal]
		if !ok {
			return "", fmt.Errorf("principal %q has no AWS ARN mapping", principal)
		}
		arns = append(arns, arn)
	}
	sort.Strings(arns)
	document := map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{map[string]any{
			"Effect": "Allow", "Principal": map[string]any{"AWS": arns}, "Action": "sts:AssumeRole",
		}},
	}
	data, err := json.Marshal(document)
	return string(data), err
}

func marshalIdentityPolicy(statements []scenario.AWSPolicyStatement) (string, error) {
	awsStatements := make([]map[string]any, 0, len(statements))
	for _, statement := range statements {
		actions := append([]string(nil), statement.Actions...)
		resources := append([]string(nil), statement.Resources...)
		sort.Strings(actions)
		sort.Strings(resources)
		awsStatements = append(awsStatements, map[string]any{
			"Effect": statement.Effect, "Action": actions, "Resource": resources,
		})
	}
	document := map[string]any{"Version": "2012-10-17", "Statement": awsStatements}
	data, err := json.Marshal(document)
	return string(data), err
}

type trustPolicyDocument struct {
	Statement json.RawMessage `json:"Statement"`
}

type trustStatement struct {
	Effect    string         `json:"Effect"`
	Action    stringList     `json:"Action"`
	Principal trustPrincipal `json:"Principal"`
}

type trustPrincipal struct {
	AWS stringList `json:"AWS"`
}

func (p *trustPrincipal) UnmarshalJSON(data []byte) error {
	var wildcard string
	if err := json.Unmarshal(data, &wildcard); err == nil {
		p.AWS = []string{wildcard}
		return nil
	}
	type principalAlias trustPrincipal
	var object principalAlias
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*p = trustPrincipal(object)
	return nil
}

type stringList []string

func (s *stringList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*s = list
	return nil
}

func parseTrustedAWSPrincipals(document string, candidates map[string][]string) ([]string, error) {
	decoded := document
	if !strings.HasPrefix(strings.TrimSpace(document), "{") {
		var err error
		decoded, err = url.QueryUnescape(document)
		if err != nil {
			return nil, err
		}
	}
	var policy trustPolicyDocument
	if err := json.Unmarshal([]byte(decoded), &policy); err != nil {
		return nil, err
	}
	var statements []trustStatement
	if err := json.Unmarshal(policy.Statement, &statements); err != nil {
		var statement trustStatement
		if singleErr := json.Unmarshal(policy.Statement, &statement); singleErr != nil {
			return nil, err
		}
		statements = []trustStatement{statement}
	}
	allowed := make(map[string]bool, len(candidates))
	denied := make(map[string]bool, len(candidates))
	for _, statement := range statements {
		if !matchesAssumeRoleAction(statement.Action) {
			continue
		}
		for candidate, aliases := range candidates {
			if !matchesPrincipal(statement.Principal.AWS, aliases) {
				continue
			}
			switch statement.Effect {
			case "Allow":
				allowed[candidate] = true
			case "Deny":
				denied[candidate] = true
			}
		}
	}
	principals := make([]string, 0, len(candidates))
	for principal := range candidates {
		if allowed[principal] && !denied[principal] {
			principals = append(principals, principal)
		}
	}
	sort.Strings(principals)
	return principals, nil
}

func matchesAssumeRoleAction(actions []string) bool {
	for _, action := range actions {
		if action == "*" || strings.EqualFold(action, "sts:AssumeRole") || strings.EqualFold(action, "sts:*") {
			return true
		}
	}
	return false
}

func matchesPrincipal(policyPrincipals, aliases []string) bool {
	for _, policyPrincipal := range policyPrincipals {
		if policyPrincipal == "*" {
			return true
		}
		for _, alias := range aliases {
			if policyPrincipal == alias {
				return true
			}
		}
	}
	return false
}

func trustEdgeID(from, to string) string {
	sum := sha256.Sum256([]byte(from + "\x00" + to))
	return "aws-trust:" + hex.EncodeToString(sum[:8])
}

func webTrustEdgeID(from, to, audience string) string {
	sum := sha256.Sum256([]byte(from + "\x00" + to + "\x00" + audience))
	return "aws-web-trust:" + hex.EncodeToString(sum[:8])
}
