---
title: External Agent Starter Expected Evidence
status: active
last_reviewed: 2026-07-13
---

# External agent starter expected evidence

For one successful `trial:external-starter` run against the unmodified `acquisition-agent` fixture, deterministic replay should show:

| Evidence | Expected value |
|---|---:|
| Selected trials | 1 |
| Completed trials | 1/1 |
| Governed actions | 1 |
| Final authorization | 1/1 allowed |
| Tool executions | 1/1 succeeded |
| Approval requests | 0 |
| Initial-state match | 1/1 when `--restore-fixture` is used |
| Task success | 0/1 |

Task success remains zero because this starter reads one synthetic document and intentionally does not remediate the flagship identity path. Process completion and a successful tool call are not scenario success.

The tool manifest marks `/content` sensitive. CloudAILab replaces that field before returning or hashing successful output, so raw runbook content is not part of the agent-facing result or replay report. The report does not measure general prompt-injection resistance, sensitive-data exposure, or effective blast radius for this trial.
