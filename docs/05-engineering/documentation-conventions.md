---
title: Documentation Conventions
status: active
last_reviewed: 2026-07-13
---

# Documentation conventions

CloudAILab documentation is simultaneously public repository documentation and an Obsidian vault. Portability, accurate ownership, and reviewable diffs take precedence over Obsidian-only features.

## File and navigation rules

- Open the repository root as the vault. Keep product and engineering notes under `docs/`; keep standard community files such as `README.md`, `CONTRIBUTING.md`, and `SECURITY.md` at the repository root.
- Use lowercase kebab-case filenames except for community-standard filenames and ADR numbers.
- Use standard relative Markdown links with an explicit `.md` extension. Use percent-encoding when a path contains spaces.
- Do not commit Wikilinks, Obsidian URIs, `file://` navigation links, block references, or developer-machine absolute paths. A literal `file://` argument in an executable command example is allowed when the target is part of that workflow.
- Link to the document that owns the fact. Avoid duplicating milestone status, requirement text, compatibility claims, or security decisions across notes.
- Use headings as stable link targets only when a file-level link is insufficient. Update inbound links when renaming a heading.
- Use Mermaid only when a diagram communicates structure or sequence more clearly than prose or a small table. Keep the source in the Markdown document.

## Frontmatter

Every Markdown file under `docs/` and the root `README.md` starts with YAML frontmatter:

```yaml
---
title: Human-readable title
status: draft
last_reviewed: 2026-07-13
---
```

- `title` and `status` are required.
- Use the document-lifecycle statuses defined in the [master plan](../00-project/master-plan.md#documentation-system): `draft`, `proposed`, `accepted`, `active`, `deprecated`, or `superseded`.
- Use `accepted` for accepted ADRs. Use `active` for maintained guides, contracts, compatibility records, plans, and policies. Record feature or milestone completion in the document body and evidence links, not in `status`.
- Add `last_reviewed` when time-sensitive claims, external sources, compatibility, installation, security posture, or active planning are involved. Use an ISO 8601 date.
- Add specialized properties only when a documented consumer uses them. Do not add decorative tags or duplicate headings as metadata.

Root governance and community files other than `README.md` may omit frontmatter so they retain their conventional GitHub form. Third-party license material must remain unchanged.

## Capability and planning language

- The README describes only verified value, status, quick start, and navigation.
- Requirements define externally observable obligations with stable identifiers.
- The master plan defines future work and delivery order. Mark planned behavior explicitly.
- Compatibility records define exact provider, protocol, isolation, report, and evaluation fidelity. Do not generalize beyond them.
- Accepted ADRs are historical decision records. Supersede them with a new ADR instead of rewriting the old decision.
- Optional AI output cannot be presented as authoritative authorization, verification, or scoring.

## Assets and generated material

- Put pasted or dropped documentation assets under `docs/assets/`; use descriptive lowercase kebab-case names.
- Prefer text-native diagrams and examples. Commit a binary asset only when it adds information that cannot be represented clearly in portable Markdown.
- Do not commit workspace state, appearance preferences, Sync configuration, hotkeys, bookmarks, community-plugin state, caches, generated reports, or local CloudAILab state.
- Generated documentation must identify its generator and source and be reproducible.

## Review checklist

1. Does the note update the owning source of truth rather than create a competing one?
2. Are implemented, proposed, illustrative, and unsupported behaviors unambiguous?
3. Do local links resolve from both Obsidian and GitHub?
4. Are external factual claims supported by primary sources near the claim and registered when version-sensitive?
5. Are commands current, safe, and tested when practical?
6. Did the change require requirements, ADR, threat-model, compatibility, changelog, or release-bundle updates?
7. Does `go run ./internal/tools/doccheck .` pass?
