---
name: Deep Architecture Audit
description: "Perform a deep architecture and execution-flow audit with CLI-startup-first tracing, then command-specific path analysis."
agent: "Codebase Flow Reviewer"
argument-hint: "Scope and objective (e.g., full repo, auth path, config lifecycle, command execution)"
---
Perform a deep architecture audit for the selected scope.

Requirements:

* Prioritize CLI startup and initialization path first.
* Expand into command-specific flows, including branching behavior.
* Cover file-by-file or module-group responsibilities in depth.
* Identify side effects, external integrations, state transitions, and error pivots.
* Explicitly mark assumptions and confidence level where certainty is limited.

Output sections:

1. Entry Points
1. CLI Startup and Initialization Flow
1. Command-Specific Flows
1. File and Module Responsibilities
1. Decision Points and Side Effects
1. Risks, Gaps, and Follow-up Inspections

Depth target:

* Prefer comprehensive explanations over brevity.
* Include concrete file and symbol citations for each major claim.
* Linking to line numbers is _too_ detailed. Since codebases change over time, it's better to link to the files, but without the specific line numbers.
