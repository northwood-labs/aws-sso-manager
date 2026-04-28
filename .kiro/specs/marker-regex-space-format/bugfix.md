# Bugfix Requirements Document

## Introduction

The marker detection regex in `config-manager/internal/marker/lexer.go` only matches the parenthesized marker format (`@config-manager:start(name)`) but all real-world configuration files in the repository use the space-separated format (`@config-manager:start name`). This causes `DetectMarkers` to return zero markers for every file that uses the space-separated format, which cascades through `ParseBlocks`, `ReplaceBlocks`, and `SyncFile` — resulting in the sync engine reporting files as "Unchanged" even when source content should be merged into the destination.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN a marker line uses the space-separated format (e.g., `# @config-manager:start block_name`) THEN the system fails to detect the marker and `DetectMarkers` returns an empty result for that line

1.2 WHEN both source and destination files contain only space-separated markers THEN the system returns empty block maps from `ParseBlocks`, causing `ReplaceBlocks` to produce no replacements and `SyncFile` to report the file as "Unchanged"

1.3 WHEN a source file contains content between space-separated markers (e.g., a list of HTML element names between `# @config-manager:start html_elements` and `# @config-manager:end html_elements`) THEN the system fails to merge that content into the corresponding destination block, leaving the destination block empty

### Expected Behavior (Correct)

2.1 WHEN a marker line uses the space-separated format (e.g., `# @config-manager:start block_name`) THEN the system SHALL detect the marker and return a valid `MarkerLine` with the correct `BlockName` and `IsStart`/`IsEnd` classification

2.2 WHEN both source and destination files contain space-separated markers THEN the system SHALL parse complete block ranges from `ParseBlocks` and `ReplaceBlocks` SHALL merge source block content into destination blocks, with `SyncFile` reporting the file as "Changed" when content differs

2.3 WHEN a source file contains content between space-separated markers THEN the system SHALL extract and merge that content into the matching destination block, identical to how it would behave with parenthesized markers

### Unchanged Behavior (Regression Prevention)

3.1 WHEN a file contains no marker lines at all THEN the system SHALL CONTINUE TO return an empty marker list from `DetectMarkers` and empty block maps from `ParseBlocks`

3.2 WHEN a line contains `@config-manager:` but is not followed by `start` or `end` with a valid block name THEN the system SHALL CONTINUE TO ignore that line as a non-marker

3.3 WHEN marker block names contain alphanumeric characters, hyphens, and underscores (e.g., `html_elements`, `my-block`, `block123`) THEN the system SHALL accept those names in the space-separated format

---

### Bug Condition

```pascal
FUNCTION isBugCondition(X)
  INPUT: X of type MarkerLine (a string representing a single line of text)
  OUTPUT: boolean

  // Returns true when the line is a valid space-separated marker
  // that the current regex fails to match
  RETURN X matches pattern "^\s*(?://|#)\s*@config-manager:(start|end)\s+[A-Za-z0-9_-]+\s*$"
    AND X does NOT match pattern "^\s*(?://|#)\s*@config-manager:(start|end)\([A-Za-z0-9_-]+\)\s*$"
END FUNCTION
```

### Fix Checking Property

```pascal
// Property: Fix Checking — Space-separated markers are detected
FOR ALL X WHERE isBugCondition(X) DO
  result ← DetectMarkers'([X])
  ASSERT len(result) = 1
    AND result[0].BlockName = extractBlockName(X)
    AND result[0].IsStart = containsStart(X)
END FOR
```

### Preservation Checking Property

```pascal
// Property: Preservation Checking — Non-marker lines still ignored
FOR ALL X WHERE NOT isBugCondition(X) DO
  ASSERT DetectMarkers([X]) = DetectMarkers'([X])
END FOR
```
