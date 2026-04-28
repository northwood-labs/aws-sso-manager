# Implementation Plan

- [x] 1. Write bug condition exploration test
  - **Property 1: Bug Condition** - Space-Separated Markers Not Detected
  - **CRITICAL**: This test MUST FAIL on unfixed code — failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior — it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate the bug exists
  - **Scoped PBT Approach**: For any valid block name (`[A-Za-z0-9_-]+`) and any supported comment prefix (`//` or `#`), construct a space-separated marker line and assert `DetectMarkers` returns the correct `MarkerLine`
  - Create a new test function `TestProperty_BugCondition_SpaceSeparatedMarkersDetected` in `config-manager/internal/marker/lexer_property_test.go`
  - Use `pgregory.net/rapid` to generate random block names via `rapid.StringMatching("[A-Za-z0-9_-]{1,30}")` and random comment prefixes (`//` or `#`)
  - Construct start lines as `fmt.Sprintf("%s @config-manager:start %s", prefix, blockName)` and end lines as `fmt.Sprintf("%s @config-manager:end %s", prefix, blockName)`
  - Assert `DetectMarkers([]string{startLine, endLine})` returns exactly 2 markers
  - Assert `markers[0].IsStart == true`, `markers[0].BlockName == blockName`, `markers[0].LineNum == 0`
  - Assert `markers[1].IsStart == false`, `markers[1].BlockName == blockName`, `markers[1].LineNum == 1`
  - Also include optional leading whitespace via `rapid.StringMatching("[ \\t]{0,4}")` to test indented markers
  - Run test on UNFIXED code with `go test ./internal/marker/... -run TestProperty_BugCondition` from `config-manager/`
  - **EXPECTED OUTCOME**: Test FAILS (this is correct — it proves the bug exists because the current regex requires parentheses)
  - Document counterexamples found: `DetectMarkers` returns 0 markers for all space-separated inputs because `markerRegex` requires literal `\(` and `\)` around the block name
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 2.1_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** - Non-Marker Lines Still Ignored
  - **IMPORTANT**: Follow observation-first methodology
  - Create a new test function `TestProperty_Preservation_NonMarkerLinesIgnored` in `config-manager/internal/marker/lexer_property_test.go`
  - Use `pgregory.net/rapid` to generate random non-marker lines that do NOT satisfy the bug condition
  - Generate lines from these categories using `rapid.OneOf`:
    - Plain comments: `rapid.StringMatching("# [A-Za-z ]{1,40}")` (no `@config-manager:` prefix)
    - Blank lines: `rapid.Just("")`
    - Code lines: `rapid.StringMatching("[a-z_]+ = [0-9]+")` (no comment prefix)
    - Invalid marker-like lines: `rapid.StringMatching("# @config-manager:(update|delete) [A-Za-z0-9_-]+")` (invalid action keyword)
    - Incomplete markers: `rapid.StringMatching("# @config-manager:start$")` (missing block name)
  - Observe: `DetectMarkers([]string{nonMarkerLine})` returns empty slice on UNFIXED code
  - Write property-based test asserting `len(DetectMarkers([]string{line})) == 0` for all generated non-marker lines
  - Run tests on UNFIXED code with `go test ./internal/marker/... -run TestProperty_Preservation` from `config-manager/`
  - **EXPECTED OUTCOME**: Tests PASS (this confirms baseline behavior to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2_

- [x] 3. Fix for space-separated marker regex not matching production marker format

  - [x] 3.1 Implement the fix
    - In `config-manager/internal/marker/lexer.go`, change the `markerRegex` pattern from `^\s*(?://|#)\s*@config-manager:(start|end)\(([A-Za-z0-9_-]+)\)\s*$` to `^\s*(?://|#)\s*@config-manager:(start|end)\s+([A-Za-z0-9_-]+)\s*$`
    - The block name remains in capture group 2 — no changes to `DetectMarkers` logic needed
    - Update the package-level doc comment from `@config-manager:start(name)` and `@config-manager:end(name)` to `@config-manager:start name` and `@config-manager:end name`
    - Update the `markerRegex` variable doc comment to document the space-separated format
    - Update the `DetectMarkers` function doc comment to reference the space-separated format
    - Update the existing `TestProperty_MarkerDetectionAcrossCommentStyles` test in `lexer_property_test.go` to use space-separated format (`fmt.Sprintf("%s%s @config-manager:start %s", ...)`) instead of parenthesized format (`fmt.Sprintf("%s%s @config-manager:start(%s)", ...)`)
    - Update any other existing tests in `lexer_test.go` or `parser_test.go` that use parenthesized marker format to use space-separated format
    - No changes needed to `parser.go`, `replace.go`, or `sync_file.go` — they consume `MarkerLine` structs and are format-agnostic
    - _Bug_Condition: isBugCondition(input) where input matches space-separated format `^\s*(?://|#)\s*@config-manager:(start|end)\s+[A-Za-z0-9_-]+\s*$`_
    - _Expected_Behavior: DetectMarkers returns correct MarkerLine with accurate BlockName, IsStart/IsEnd, and LineNum for all space-separated markers_
    - _Preservation: Non-marker lines (plain comments, code, blank lines, invalid marker-like lines) continue to be ignored by DetectMarkers_
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 3.1, 3.2, 3.3_

  - [x] 3.2 Verify bug condition exploration test now passes
    - **Property 1: Expected Behavior** - Space-Separated Markers Detected
    - **IMPORTANT**: Re-run the SAME test from task 1 — do NOT write a new test
    - The test from task 1 encodes the expected behavior
    - When this test passes, it confirms the expected behavior is satisfied
    - Run `go test ./internal/marker/... -run TestProperty_BugCondition` from `config-manager/`
    - **EXPECTED OUTCOME**: Test PASSES (confirms bug is fixed — space-separated markers are now detected)
    - _Requirements: 2.1, 2.3_

  - [x] 3.3 Verify preservation tests still pass
    - **Property 2: Preservation** - Non-Marker Lines Still Ignored
    - **IMPORTANT**: Re-run the SAME tests from task 2 — do NOT write new tests
    - Run `go test ./internal/marker/... -run TestProperty_Preservation` from `config-manager/`
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions — non-marker lines are still ignored)
    - Confirm all tests still pass after fix (no regressions)

- [x] 4. Checkpoint - Ensure all tests pass
  - Run full marker package test suite: `go test ./internal/marker/...` from `config-manager/`
  - Verify all property tests pass: `TestProperty_BugCondition_SpaceSeparatedMarkersDetected`, `TestProperty_Preservation_NonMarkerLinesIgnored`, `TestProperty_MarkerDetectionAcrossCommentStyles`, `TestProperty_MultiBlockParsingReturnsAllBlocks`
  - Verify all existing unit tests pass (including any updated tests that now use space-separated format)
  - Ensure all tests pass, ask the user if questions arise.
