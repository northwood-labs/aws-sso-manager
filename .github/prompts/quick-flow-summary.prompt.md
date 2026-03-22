---
name: Quick Flow Summary
description: "Generate a concise codebase flow summary from entrypoint to major modules. Use when you need a fast orientation."
agent: "Codebase Flow Reviewer"
argument-hint: "Target scope (repo/package/path) and any specific flow to prioritize"
---
Produce a concise flow summary of the selected scope.

Requirements:

* Start with the main entrypoint and immediate downstream calls.
* Focus on the highest-impact control-flow path only.
* Summarize module responsibilities in short bullets.
* Call out unknowns or dynamic dispatch points.

Output sections:

1. Entry Point
1. Primary Flow (5-10 steps)
1. Module Roles (short bullets)
1. Risks or Unknowns

Keep the answer concise and practical for quick onboarding.
