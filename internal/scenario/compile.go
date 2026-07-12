package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func Compile(s Scenario, seed int64) (Compiled, error) {
	if err := Validate(s); err != nil {
		return Compiled{}, err
	}

	nodes := make([]Node, 0, len(s.Spec.Tenants)+len(s.Spec.Principals)+len(s.Spec.Resources))
	for _, tenant := range s.Spec.Tenants {
		nodes = append(nodes, Node{
			ID: tenant.ID, Kind: "tenant", Type: "organization", DisplayName: tenant.Name,
		})
	}
	for _, principal := range s.Spec.Principals {
		nodes = append(nodes, Node{
			ID: principal.ID, Kind: "principal", Tenant: principal.Tenant,
			Type: principal.Type, DisplayName: principal.DisplayName,
		})
	}
	for _, resource := range s.Spec.Resources {
		nodes = append(nodes, Node{
			ID: resource.ID, Kind: "resource", Tenant: resource.Tenant,
			Type: resource.Type, DisplayName: resource.DisplayName,
			Classification: resource.Classification,
		})
	}

	edges := make([]Relationship, len(s.Spec.Relationships))
	for i, edge := range s.Spec.Relationships {
		edges[i] = edge
		edges[i].Actions = append([]string(nil), edge.Actions...)
	}
	invariants := append([]Invariant(nil), s.Spec.Verification.Invariants...)
	objectives := append([]Objective(nil), s.Spec.Objectives...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	sort.Slice(invariants, func(i, j int) bool { return invariants[i].ID < invariants[j].ID })
	sort.Slice(objectives, func(i, j int) bool { return objectives[i].ID < objectives[j].ID })
	for i := range edges {
		sort.Strings(edges[i].Actions)
	}

	compiled := Compiled{
		SchemaVersion: APIVersion,
		ScenarioName:  s.Metadata.Name, ScenarioVersion: s.Metadata.Version,
		Title: s.Metadata.Title, Seed: seed, Briefing: s.Spec.Briefing,
		Objectives: objectives, Nodes: nodes, Edges: edges, Invariants: invariants,
	}
	digest, err := digestCompiled(compiled)
	if err != nil {
		return Compiled{}, err
	}
	compiled.Digest = digest
	return compiled, nil
}

func digestCompiled(compiled Compiled) (string, error) {
	compiled.Digest = ""
	data, err := json.Marshal(compiled)
	if err != nil {
		return "", fmt.Errorf("marshal compiled scenario: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
