---
name: Review Output Format Standards
description: "Use when producing code reviews, architecture walkthroughs, call-flow analyses, or execution-path documentation to standardize file references and flow diagrams."
---

Use this format whenever you produce review or flow-analysis output.

## File References

* Use markdown links for every concrete file mention.
* Include line anchors when citing specific logic.
* Preferred style:
  * File only: `[cmd/root.go](cmd/root.go)`
  * File with line: `[cmd/root.go#L12](cmd/root.go#L12)`
* Do not cite line numbers without links.
* Keep references precise and tied to specific claims.

## Flow Diagrams

* Provide a Mermaid diagram when describing non-trivial flows.
* Keep diagrams directional, readable, and close to the narrative.
* Use short node labels and include key branch points.

Template:

```mermaid
flowchart TD
  A[Entrypoint] --> B[Init]
  B --> C{Branch}
  C -->|Path 1| D[Handler A]
  C -->|Path 2| E[Handler B]
```

## Review Structure

* Findings first for review tasks, ordered by severity.
* Follow with open questions/assumptions.
* End with concise summary or next checks.

## Evidence Quality

* Ground claims in observed code only.
* Mark uncertainty explicitly when behavior is inferred.
* Distinguish confirmed behavior from assumptions.
