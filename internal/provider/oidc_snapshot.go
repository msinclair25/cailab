package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/msinclair25/cailab/internal/scenario"
)

// snapshotOIDC normalizes the declared client-to-audience contract into the
// canonical graph. Runtime token validation remains authoritative for issued
// tokens; these edges describe which exchanges the active scenario permits.
func snapshotOIDC(compiled scenario.Compiled) scenario.Compiled {
	if compiled.Providers.OIDC == nil {
		return compiled
	}
	edges := make([]scenario.Relationship, 0, len(compiled.Edges))
	for _, edge := range compiled.Edges {
		if !strings.HasPrefix(edge.ID, "oidc:audience:") {
			edges = append(edges, edge)
		}
	}
	for _, client := range compiled.Providers.OIDC.Clients {
		for _, audience := range client.Audiences {
			edges = append(edges, scenario.Relationship{
				ID:   oidcAudienceEdgeID(client.Node, audience.Node, audience.Value),
				From: client.Node, To: audience.Node, Type: "can_access",
				Actions: []string{"oidc:TokenExchange"},
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	compiled.Edges = edges
	return compiled
}

func oidcAudienceEdgeID(clientNode, audienceNode, audience string) string {
	sum := sha256.Sum256([]byte(clientNode + "\x00" + audienceNode + "\x00" + audience))
	return "oidc:audience:" + hex.EncodeToString(sum[:8])
}
