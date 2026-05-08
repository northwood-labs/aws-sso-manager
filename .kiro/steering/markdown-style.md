---
inclusion: fileMatch
fileMatchPattern: "**/*.md"
---

# Markdown Style Guide

This project enforces markdown conventions via `.rumdl.toml` and `.editorconfig`. Follow these rules when writing or editing any `.md` file.

## Indentation and whitespace

<!-- @config-manager:start whitespace -->
* Use 2-space indentation for markdown files (per `.editorconfig`).
* Use LF line endings, no trailing whitespace.
* End every file with a single trailing newline.
* Maximum one consecutive blank line (no double blanks).
* One blank line above and below headings, lists, tables, fenced code blocks, and horizontal rules.
* No multiple consecutive spaces within text content.
* Sentences must start with a capital letter.
<!-- @config-manager:end whitespace -->

## Headings

<!-- @config-manager:start headings -->
* Use ATX style (`#` prefix), not Setext (underlines).
* Start the document with a single `# H1` heading. Only one H1 per file.
* Increment heading levels by one — no skipping (e.g., `##` → `####` is invalid).
* Headings must start at column 0 (no indentation).
* No trailing punctuation (`.`, `,`, `;`, `:`) on headings.
* One space after `#` — no multiple spaces.
* Do not use emphasis (bold/italic) as a substitute for headings.
* Duplicate headings are allowed only among siblings (same parent level), not across the entire document.
* The `# H1` heading should use Title Case formatting (lowercase words: `a`, `an`, `and`, `as`, `at`, `but`, `by`, `for`, `in`, `into`, `nor`, `of`, `on`, `onto`, `or`, `so`, `the`, `to`, `up`, `upon`, `via`, `vs`, `with`, `without`, `yet`).
* All other headings (`## H2`, `### H3`, `#### H4`, `##### H5`, `###### H6`) should use sentence case formatting.
<!-- @config-manager:end headings -->

## Emphasis and strong

<!-- @config-manager:start em-strong -->
* Use _underscores_ for emphasis (italic): `_text_`, not `*text*`.
* Use **asterisks** for strong (bold): `**text**`, not `__text__`.
* No spaces inside emphasis or strong markers.
<!-- @config-manager:end em-strong -->

## Lists

<!-- @config-manager:start lists -->
* Use `*` (asterisk) for unordered list markers, not `-` or `+`.
* Use ordered numbering for ordered lists (`1.`, `2.`, `3.`), not all-ones.
* One space after the list marker.
* Indent nested unordered lists by 2 spaces (matching `ul-indent` config).
* Blank line before and after list blocks.
* Use consistent spacing within list items (all loose or all tight).
<!-- @config-manager:end lists -->

## Code

<!-- @config-manager:start code-pre -->
* Use fenced code blocks (triple backticks), not indented blocks.
* Use backtick fences, not tilde fences.
* Always specify a language identifier on fenced code blocks (e.g., ` ```go `, ` ```text `, ` ```bash `).
* Use ` ```text ` for plain-text diagrams, pipeline flows, and non-executable pseudocode.
* No spaces inside inline code spans.
* Dollar-sign prefixed commands in code blocks should show output or use a non-prefixed style.
<!-- @config-manager:end code-pre -->

## Tables

<!-- @config-manager:start tables -->
* Use leading and trailing pipes on every row.
* Align column delimiters (pad cells with spaces so pipes line up).
* Blank line before and after tables.
* Every row must have the same number of columns.

Example:

```markdown
| Column A | Column B |
|----------|----------|
| value    | value    |
```

<!-- @config-manager:end tables -->

## Links and images

<!-- @config-manager:start links-images -->
* Use descriptive link text. Prohibited: "click here", "here", "link", "more".
* No bare URLs — wrap in angle brackets or use `[text](url)` syntax.
* No reversed link syntax (`(url)[text]`).
* No spaces inside link brackets or parentheses.
* All images must have alt text.
* Relative links must point to existing files.
* Reference-style link definitions must be used if defined.
<!-- @config-manager:end links-images -->

## Proper names

<!-- @config-manager:start proper-names -->
These terms must use exact casing wherever they appear in prose (not enforced inside code blocks or HTML):

* JSON
* macOS
* Northwood Labs
* OpenTofu
* Terraform
* Terragrunt
* Terratest
* TOML
* YAML
<!-- @config-manager:end proper-names -->

## HTML

<!-- @config-manager:start html -->
Inline HTML is restricted to these allowed elements: `a`, `b`, `br`, `code`, `details`, `div`, `img`, `li`, `nobr`, `ol`, `p`, `pre`, `span`, `summary`, `ul`. All other HTML elements will trigger a lint warning.
<!-- @config-manager:end html -->

## Blockquotes

<!-- @config-manager:start blockquotes -->
* No blank lines inside blockquotes.
* No multiple spaces after `>`.
* Consecutive blockquotes should be separated by a horizontal rule.
<!-- @config-manager:end blockquotes -->

## Horizontal rules

<!-- @config-manager:start hr -->
* Use the `---` style.
* Should be used sparingly.
* Do not use them immediately before headings.
<!-- @config-manager:end hr -->

## Footnotes

<!-- @config-manager:start footnotes -->
* All footnote references must have a corresponding definition.
* Footnote definitions must appear in the order they are referenced.
* No empty footnote definitions.
<!-- @config-manager:end footnotes -->

## Front matter

<!-- @config-manager:start frontmatter -->
* Include a blank line after YAML front matter closing `---`.
<!-- @config-manager:end frontmatter -->

## Spec document patterns

This project uses structured spec documents under `.kiro/specs/`. Follow these patterns:

### Requirements.md

<!-- @config-manager:start spec-requirements -->
* Start with `# Requirements Document`.
* Include `## Introduction`, `## Glossary`, and `## Requirements` sections.
* Each requirement gets `### Requirement N: Title` with a `**User story:**` line and `#### Acceptance criteria` numbered list.
* Glossary entries use `* **Term**: Definition` format.
* Acceptance criteria use WHEN/THEN/SHALL phrasing.
<!-- @config-manager:end spec-requirements -->

### Design.md

<!-- @config-manager:start spec-design -->
* Start with `# Design Document: Feature Name`.
* Include `## Overview`, `## Architecture`, `## Components and Interfaces`, `## Data Models`, `## Correctness Properties`, `## Error Handling`, `## Testing Strategy`.
* Use mermaid code blocks (` ```mermaid `) for architecture diagrams.
* Use tables for impact analysis, supported styles, and test matrices.
* Properties use _italic_ universal quantifiers: _For any_ valid input...
<!-- @config-manager:end spec-design -->

### Tasks.md

<!-- @config-manager:start spec-tasks -->
* Start with `# Implementation Plan: Feature Name`.
* Include `## Overview` and `## Tasks` sections.
* Use `* [ ] N. Task title` for top-level tasks (asterisk list marker, checkbox).
* Use `  * [ ] N.M Subtask title` for subtasks (2-space indent). <!-- markdownlint-disable-line MD038 -->
* Bullet points under subtasks describe implementation details.
* End subtasks with `* _Validates: Requirements X.Y, X.Z._` for traceability.
* Checkpoint tasks have no subtasks — just a description paragraph.
<!-- @config-manager:end spec-tasks -->
