# Marker Regex Space-Separated Format Bugfix Design

## Overview

The `markerRegex` in `config-manager/internal/marker/lexer.go` only matches parenthesized markers (`@config-manager:start(name)`) but all real-world configuration files use space-separated markers (`@config-manager:start name`). This causes `DetectMarkers` to return zero markers for every production file, cascading through `ParseBlocks` and `ReplaceBlocks` so that `SyncFile` reports every file as "Unchanged" even when source content should be merged.

The fix is a single regex change in `lexer.go` that accepts both the parenthesized format and the space-separated format. No other files require modification because downstream consumers (`ParseBlocks`, `ReplaceBlocks`, `SyncFile`) operate on `MarkerLine` structs produced by `DetectMarkers`.

## Glossary

- **Bug_Condition (C)**: A marker line that uses the space-separated format (`# @config-manager:start block_name`) — valid in production but not matched by the current regex
- **Property (P)**: `DetectMarkers` returns a correct `MarkerLine` (with accurate `BlockName`, `IsStart`/`IsEnd`, and `LineNum`) for every space-separated marker line
- **Preservation**: Parenthesized markers (`# @config-manager:start(block_name)`) continue to be detected identically; non-marker lines continue to be ignored
- **`markerRegex`**: The compiled `regexp.Regexp` in `lexer.go` that identifies marker comment lines
- **`DetectMarkers`**: The function in `lexer.go` that scans a slice of lines and returns `[]MarkerLine`
- **`MarkerLine`**: Struct with `LineNum`, `BlockName`, and `IsStart` fields representing a detected marker
- **`ParseBlocks`**: Function in `parser.go` that pairs start/end `MarkerLine` entries into `BlockRange` maps
- **`ReplaceBlocks`**: Function in `replace.go` that splices source block content into destination lines

## Bug Details

### Bug Condition

The bug manifests when a marker line uses a space (instead of parentheses) to separate the action keyword from the block name. The current `markerRegex` requires literal `(` and `)` around the block name, so any space-separated marker is silently skipped by `DetectMarkers`.

**Formal Specification:**
```
FUNCTION isBugCondition(input)
  INPUT: input of type string (a single line of text)
  OUTPUT: boolean

  RETURN input matches "^\s*(?://|#)\s*@config-manager:(start|end)\s+[A-Za-z0-9_-]+\s*$"
    AND input does NOT match "^\s*(?://|#)\s*@config-manager:(start|end)\([A-Za-z0-9_-]+\)\s*$"
END FUNCTION
```

### Examples

- `# @config-manager:start html_elements` → **Current**: not detected (bug). **Expected**: `MarkerLine{BlockName: "html_elements", IsStart: true}`
- `# @config-manager:end html_elements` → **Current**: not detected (bug). **Expected**: `MarkerLine{BlockName: "html_elements", IsStart: false}`
- `# @config-manager:start spelling` → **Current**: not detected (bug). **Expected**: `MarkerLine{BlockName: "spelling", IsStart: true}`
- `// @config-manager:start my-block` → **Current**: not detected (bug). **Expected**: `MarkerLine{BlockName: "my-block", IsStart: true}`
- Parenthesized markers (`# @config-manager:start(name)` / `# @config-manager:end(name)`) were never meant to be supported. Support for this pattern can be safely removed.

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- `ParseBlocks` must continue to pair start/end markers into `BlockRange` maps without errors for valid marker pairs
- `ReplaceBlocks` must continue to splice source content into destination blocks
- Non-marker lines (plain comments, code, blank lines) must continue to be ignored by `DetectMarkers`
- Lines containing `@config-manager:` but with an invalid action (not `start` or `end`) or missing block name must continue to be ignored
- Block names containing alphanumeric characters, hyphens, and underscores must continue to be accepted

**Scope:**
All inputs that do NOT use the space-separated marker format should be completely unaffected by this fix. This includes:
- Non-marker comment lines
- Code lines, blank lines, and any other file content
- Invalid marker-like lines (e.g., `# @config-manager:update foo`, `# @config-manager:start`)

## Hypothesized Root Cause

Based on the bug description and code analysis, the root cause is a single issue:

1. **Regex Only Matches Parenthesized Format**: The `markerRegex` pattern is:
   ```
   ^\s*(?://|#)\s*@config-manager:(start|end)\(([A-Za-z0-9_-]+)\)\s*$
   ```
   The `\(` and `\)` require literal parentheses around the block name. Space-separated markers like `# @config-manager:start html_elements` have no parentheses, so `FindStringSubmatch` returns `nil` and the line is skipped.

2. **No Alternative Branch**: The regex has no alternation (`|`) or optional group to match the space-separated format as an alternative to parentheses.

3. **Downstream Code Is Not At Fault**: `ParseBlocks`, `ReplaceBlocks`, and `SyncFile` all operate on `MarkerLine` structs. They are format-agnostic — they never see the raw line text. The bug is entirely in the lexer's regex.

## Correctness Properties

Property 1: Bug Condition - Space-separated markers are detected

_For any_ valid block name (matching `[A-Za-z0-9_-]+`) and any supported comment prefix (`//` or `#`), constructing a space-separated marker line (`prefix @config-manager:start name` or `prefix @config-manager:end name`) SHALL result in `DetectMarkers` returning exactly one `MarkerLine` with the correct `BlockName`, correct `IsStart`/`IsEnd` classification, and correct `LineNum`.

**Validates: Requirements 2.1, 2.3**

Property 2: Preservation - Non-marker lines still ignored

_For any_ line that is not a valid space-separated marker (non-marker comments, code, blank lines, invalid marker-like lines), `DetectMarkers` SHALL continue to return no markers for that line, identical to the behavior before the fix.

**Validates: Requirements 3.3, 3.4, 3.5**

## Fix Implementation

### Changes Required

Assuming our root cause analysis is correct:

**File**: `config-manager/internal/marker/lexer.go`

**Variable**: `markerRegex`

**Specific Changes**:

1. **Replace the regex to match space-separated format**: Replace the parenthesized capture group `\(([A-Za-z0-9_-]+)\)` with a space-separated capture `\s+([A-Za-z0-9_-]+)`. The parenthesized format was never intended for production use and can be removed.

   Current regex:
   ```
   ^\s*(?://|#)\s*@config-manager:(start|end)\(([A-Za-z0-9_-]+)\)\s*$
   ```

   Fixed regex:
   ```
   ^\s*(?://|#)\s*@config-manager:(start|end)\s+([A-Za-z0-9_-]+)\s*$
   ```

   The block name remains in capture group 2. No alternation or extra groups needed.

2. **No changes to `DetectMarkers` logic**: The block name stays in `matches[2]`, so the existing `BlockName: matches[2]` line is unchanged.

3. **Update the package doc comment**: The package-level comment references `@config-manager:start(name)`. Update it to `@config-manager:start name` and `@config-manager:end name`.

4. **Update the `markerRegex` doc comment**: The variable comment should document the space-separated format.

5. **No changes to other files**: `parser.go`, `replace.go`, and `sync_file.go` consume `MarkerLine` structs and are format-agnostic.

## Testing Strategy

### Validation Approach

The testing strategy follows a two-phase approach: first, surface counterexamples that demonstrate the bug on unfixed code, then verify the fix works correctly and preserves existing behavior.

### Exploratory Bug Condition Checking

**Goal**: Surface counterexamples that demonstrate the bug BEFORE implementing the fix. Confirm that the root cause is the regex pattern. If space-separated markers fail to match, the root cause is confirmed.

**Test Plan**: Write tests that construct space-separated marker lines and call `DetectMarkers`. Run these tests on the UNFIXED code to observe failures.

**Test Cases**:
1. **Hash-comment space-separated start**: `# @config-manager:start html_elements` — assert `DetectMarkers` returns 1 marker (will fail on unfixed code)
2. **Hash-comment space-separated end**: `# @config-manager:end html_elements` — assert `DetectMarkers` returns 1 marker (will fail on unfixed code)
3. **Slash-comment space-separated start**: `// @config-manager:start my-block` — assert `DetectMarkers` returns 1 marker (will fail on unfixed code)
4. **Real-world file content**: Pass the actual lines from `.github/configs/templates/markdownlint.config.toml` — assert markers are detected (will fail on unfixed code)

**Expected Counterexamples**:
- `DetectMarkers` returns 0 markers for all space-separated inputs
- Root cause confirmed: `markerRegex.FindStringSubmatch` returns `nil` because `\(` and `\)` are required

### Fix Checking

**Goal**: Verify that for all inputs where the bug condition holds, the fixed function produces the expected behavior.

**Pseudocode:**
```
FOR ALL input WHERE isBugCondition(input) DO
  result := DetectMarkers'([input])
  ASSERT len(result) = 1
  ASSERT result[0].BlockName = extractBlockName(input)
  ASSERT result[0].IsStart = containsStart(input)
END FOR
```

### Preservation Checking

**Goal**: Verify that for all inputs where the bug condition does NOT hold, the fixed function produces the same result as the original function.

**Pseudocode:**
```
FOR ALL input WHERE NOT isBugCondition(input) DO
  ASSERT DetectMarkers(input) = DetectMarkers'(input)
END FOR
```

**Testing Approach**: Property-based testing with `pgregory.net/rapid` is recommended for both fix checking and preservation checking because:
- It generates many random block names, comment prefixes, and whitespace combinations
- It catches edge cases in the regex (e.g., block names with only hyphens, single-character names)
- It provides strong guarantees that both formats work across the full input domain

**Test Plan**: The existing `lexer_property_test.go` already tests parenthesized markers via `TestProperty_MarkerDetectionAcrossCommentStyles`. Write new property tests for space-separated markers and a preservation test that verifies non-marker lines are still ignored.

**Test Cases**:
1. **Space-separated detection property**: Generate random block names and comment prefixes, construct space-separated markers, assert `DetectMarkers` returns correct `MarkerLine`
2. **Non-marker line preservation**: Generate random non-marker lines, assert `DetectMarkers` returns empty results
3. **Mixed-format file detection**: Generate files with multiple space-separated markers, verify all are detected with correct block names and line numbers

### Unit Tests

- Test space-separated markers with `#` comment prefix (TOML, YAML, Bash, Python)
- Test space-separated markers with `//` comment prefix (Go, JS, TS)
- Test leading whitespace variations with space-separated markers
- Test edge cases: single-character block names, names with only hyphens/underscores
- Test that invalid lines are still rejected (no block name, invalid action keyword)
- Test real-world file content from `.github/configs/templates/markdownlint.config.toml`

### Property-Based Tests

- Generate random valid block names and verify space-separated markers are detected correctly (fix checking)
- Generate random non-marker lines and verify they are ignored (preservation checking)
- Generate mixed-format files (multiple space-separated markers) and verify all markers are detected

### Integration Tests

- Test full `ParseBlocks` pipeline with space-separated markers — verify correct `BlockRange` maps
- Test `ReplaceBlocks` with space-separated markers in both source and destination — verify content is merged
