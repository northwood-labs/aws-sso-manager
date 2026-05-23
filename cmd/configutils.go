// Copyright 2025-2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	// ManagedStartMarkerPrefix and managedEndMarkerPrefix delimit the regions
	// of ~/.aws/config that this tool owns. Content between a matching
	// start/end pair is regenerated on every "update" — anything outside is
	// left untouched. This marker-based approach lets the tool coexist with
	// hand-edited sections.
	managedStartMarkerPrefix = "aws-sso-manager: start "
	managedEndMarkerPrefix   = "aws-sso-manager: end "

	fmtInspectingManagedMarkers = "inspecting managed markers: %w"
	fmtOpeningAWSConfigFile     = "opening AWS config file: %w"
)

// managedMarkerReport captures the structural health of managed blocks so the
// validate command can report all anomalies in a single pass rather than
// failing on the first one.
type managedMarkerReport struct {
	startCounts map[string]int
	endCounts   map[string]int
	issues      map[string][]string
	profiles    []string
}

// appendManagedMarkerIssue deduplicates issues per profile so overlapping
// anomalies (e.g., an overlap reported from both profiles' perspectives) don't
// produce duplicate error messages.
func appendManagedMarkerIssue(issues map[string][]string, profile, issue string) {
	if slices.Contains(issues[profile], issue) {
		return
	}

	issues[profile] = append(issues[profile], issue)
}

// parseManagedMarkerProfile extracts the profile name from a marker line.
// The marker format uses a fixed prefix and trailing dashes, so we strip both
// to recover the bare profile name.
func parseManagedMarkerProfile(line, prefix string) (string, bool) {
	_, after, ok := strings.Cut(line, prefix)
	if !ok {
		return "", false
	}

	rest := strings.TrimSpace(after)

	name := strings.TrimRight(strings.TrimSuffix(rest, "--------"), " -")
	if name == "" {
		return "", false
	}

	return name, true
}

// inspectManagedMarkers performs a single-pass scan of the AWS config file to
// detect structural anomalies in managed blocks: mismatched counts, duplicates,
// overlaps, orphaned end markers, and unclosed start markers. A single pass is
// important because the config file can be large and we want the validate
// command to be fast.
func inspectManagedMarkers() (*managedMarkerReport, error) { // lint:allow_complexity
	f, err := os.Open(awsConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf(fmtOpeningAWSConfigFile, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error closing file: %v\n", err)
		}
	}()

	report := &managedMarkerReport{
		startCounts: make(map[string]int),
		endCounts:   make(map[string]int),
		issues:      make(map[string][]string),
	}

	seenProfiles := make(map[string]struct{})
	activeProfile := ""

	addProfile := func(profile string) {
		if _, ok := seenProfiles[profile]; ok {
			return
		}

		seenProfiles[profile] = struct{}{}
		report.profiles = append(report.profiles, profile)
	}

	scanner := bufio.NewScanner(f)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := scanner.Text()

		if profile, ok := parseManagedMarkerProfile(line, managedStartMarkerPrefix); ok {
			addProfile(profile)

			report.startCounts[profile]++

			if activeProfile != "" {
				issue := fmt.Sprintf(
					"overlapping managed block markers at line %d: "+
						"found start marker for profile %q while profile %q block is still open",
					lineNo,
					profile,
					activeProfile,
				)

				appendManagedMarkerIssue(report.issues, activeProfile, issue)
				appendManagedMarkerIssue(report.issues, profile, issue)

				continue
			}

			activeProfile = profile

			continue
		}

		profile, ok := parseManagedMarkerProfile(line, managedEndMarkerPrefix)
		if !ok {
			continue
		}

		addProfile(profile)

		report.endCounts[profile]++

		if activeProfile == "" {
			appendManagedMarkerIssue(
				report.issues,
				profile,
				fmt.Sprintf("unmatched managed block end marker at line %d for profile %q", lineNo, profile),
			)

			continue
		}

		if activeProfile != profile {
			issue := fmt.Sprintf(
				"overlapping managed block markers at line %d: "+
					"found end marker for profile %q while profile %q block is still open",
				lineNo,
				profile,
				activeProfile,
			)

			appendManagedMarkerIssue(report.issues, activeProfile, issue)
			appendManagedMarkerIssue(report.issues, profile, issue)

			continue
		}

		activeProfile = ""
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning AWS config file: %w", err)
	}

	if activeProfile != "" {
		appendManagedMarkerIssue(
			report.issues,
			activeProfile,
			fmt.Sprintf("managed block for profile %q is left open at end of file", activeProfile),
		)
	}

	for _, profile := range report.profiles {
		starts := report.startCounts[profile]
		ends := report.endCounts[profile]

		if starts != ends {
			appendManagedMarkerIssue(
				report.issues,
				profile,
				fmt.Sprintf(
					"mismatched managed block markers for profile %q: %d start marker(s), %d end marker(s)",
					profile, starts, ends,
				),
			)
		}

		if starts > 1 {
			appendManagedMarkerIssue(
				report.issues,
				profile,
				fmt.Sprintf(
					"duplicate managed block markers for profile %q: %d blocks found (expected at most 1)",
					profile, starts,
				),
			)
		}
	}

	slices.Sort(report.profiles)

	return report, nil
}

// getProfileName generates the AWS CLI profile name for an account-role pair.
// The naming is fully configurable per SSO profile via the TOML config file,
// allowing organizations to enforce consistent naming conventions (e.g.,
// "prod-admin" instead of "My Production Account-AdministratorAccess").
// When no pattern is configured, it falls back to a safe default.
func getProfileName(profileName, account, role string) string { // lint:allow_complexity
	var (
		order     = asmConfig.GetStringSlice(profileName + ".rename.pattern.order")
		delimiter = asmConfig.GetString(profileName + ".rename.pattern.delimiter")
		prefix    = asmConfig.GetString(profileName + ".rename.prefix")
		suffix    = asmConfig.GetString(profileName + ".rename.suffix")

		accountSubstr = asmConfig.GetStringMapString(profileName + ".rename.accounts.substr_match_replace")
		roleSubstr    = asmConfig.GetStringMapString(profileName + ".rename.roles.substr_match_replace")
	)

	if len(order) == 0 {
		return buildDefaultProfileName(account, role)
	}

	if delimiter == "" {
		delimiter = "-"
	}

	var orderCopy []string

	for _, o := range order {
		switch strings.ToLower(o) {
		case "account":
			matched := false

			for match, replace := range accountSubstr {
				if !strings.Contains(strings.ToLower(account), strings.ToLower(match)) {
					continue
				}

				if replace != "" {
					orderCopy = append(orderCopy, strings.ToLower(replace))
				}

				matched = true

				continue
			}

			if !matched {
				orderCopy = append(orderCopy, strings.ToLower(account))
			}
		case "role":
			matched := false

			for match, replace := range roleSubstr {
				if !strings.Contains(strings.ToLower(role), strings.ToLower(match)) {
					continue
				}

				if replace != "" {
					orderCopy = append(orderCopy, strings.ToLower(replace))
				}

				matched = true

				continue
			}

			if !matched {
				orderCopy = append(orderCopy, strings.ToLower(role))
			}
		case "prefix":
			if prefix != "" {
				orderCopy = append(orderCopy, strings.ToLower(prefix))
			}
		case "suffix":
			if suffix != "" {
				orderCopy = append(orderCopy, strings.ToLower(suffix))
			}
		default:
			// no-op for unknown tokens.
		}
	}

	// If all tokens resolved to empty (e.g., empty prefix + empty suffix with
	// no ACCOUNT/ROLE in the order), fall back to the default so we never
	// generate a blank profile name.
	generatedName := strings.TrimSpace(strings.Join(orderCopy, delimiter))
	if generatedName == "" {
		return buildDefaultProfileName(account, role)
	}

	return generatedName
}

// buildDefaultProfileName produces a safe, filesystem-friendly profile name
// when no custom pattern is configured. It lowercases and sanitizes both tokens
// so the result works as an AWS CLI profile name without quoting or escaping.
func buildDefaultProfileName(account, role string) string {
	accountToken := toProfileToken(account)
	roleToken := toProfileToken(role)

	switch {
	case accountToken != "" && roleToken != "":
		return accountToken + "-" + roleToken
	case accountToken != "":
		return accountToken
	case roleToken != "":
		return roleToken
	default:
		return "profile"
	}
}

// toProfileToken normalizes a string into a valid, lowercase profile name
// component by replacing non-alphanumeric characters with hyphens and
// collapsing consecutive hyphens. This is idempotent:
//
// toProfileToken(toProfileToken(x)) == toProfileToken(x), which is important
// because profile names may be round-tripped through the lookup index.
func toProfileToken(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}

	b := strings.Builder{}
	lastDash := false

	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)

			lastDash = false

			continue
		}

		if !lastDash {
			b.WriteRune('-')

			lastDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

// markersExist reports whether the AWS config file already contains any managed
// block start marker for the given profile name. It is used before writing new
// markers so that orphaned markers (e.g., after manual section-header deletion)
// are caught before a duplicate block is appended.
func markersExist(profileName string) (bool, error) {
	report, err := inspectManagedMarkers()
	if err != nil {
		return false, fmt.Errorf(fmtInspectingManagedMarkers, err)
	}

	return report.startCounts[profileName] > 0, nil
}

// validateMarkers checks that the managed block for profileName is well-formed:
// exactly one start/end pair, with no mismatches or duplicates.
func validateMarkers(profileName string) error {
	report, err := inspectManagedMarkers()
	if err != nil {
		return fmt.Errorf(fmtInspectingManagedMarkers, err)
	}

	var errs []error
	for _, issue := range report.issues[profileName] {
		errs = append(errs, fmt.Errorf("%w: %s", errManagedMarkerIssue, issue))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// validateManagedMarkers checks all profiles at once, used by the validate
// command to give a comprehensive report rather than stopping at the first
// error.
func validateManagedMarkers() error {
	report, err := inspectManagedMarkers()
	if err != nil {
		return fmt.Errorf(fmtInspectingManagedMarkers, err)
	}

	var errs []error

	for _, profile := range report.profiles {
		for _, issue := range report.issues[profile] {
			errs = append(errs, fmt.Errorf("%w: %s", errManagedMarkerIssue, issue))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// getManagedSection extracts the content between a profile's managed block
// markers into a temp file. The update command uses this to load the existing
// INI sections, rebuild them from scratch, and then swap the result back in.
// Validating markers first prevents operating on a corrupt config.
func getManagedSection(profileName string) (string, error) {
	if err := validateManagedMarkers(); err != nil {
		return "", fmt.Errorf("validating managed markers: %w", err)
	}

	tmp, err := os.CreateTemp("", "aws-sso-manager-managed-*.ini")
	if err != nil {
		return "", fmt.Errorf("creating temp file for managed section: %w", err)
	}

	logger.DebugContext(context.Background(), "Reading from AWS config", logKeyFile, awsConfigFilePath)

	f, err := os.Open(awsConfigFilePath)
	if err != nil {
		return "", fmt.Errorf(fmtOpeningAWSConfigFile, err)
	}

	defer func() {
		_ = tmp.Close() // lint:allow_unhandled
		_ = f.Close()   // lint:allow_unhandled
	}()

	scanner := bufio.NewScanner(f)
	doCopy := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "aws-sso-manager: start "+profileName) {
			logger.DebugContext(context.Background(), ">| "+line)

			doCopy = true

			continue
		}

		if strings.Contains(line, "aws-sso-manager: end "+profileName) {
			logger.DebugContext(context.Background(), "<| "+line)
			break
		}

		logger.DebugContext(context.Background(), " | "+line)

		if !doCopy {
			continue
		}

		_, err = tmp.WriteString(line + "\n")
		if err != nil {
			return "", fmt.Errorf("writing to temp file: %w", err)
		}
	}

	// Check for errors.
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}

	return tmp.Name(), nil
}

// setManagedSection replaces the content between a profile's managed block
// markers with new content from tmpFile. It writes to a backup file in the same
// directory as the config so that the final [os.Rename] is an atomic
// same-filesystem operation — this prevents a half-written config if the
// process is interrupted.
func setManagedSection(tmpFile, profileName string) (string, error) { // lint:allow_complexity
	// Create the backup in the same directory as the config file so that
	// os.Rename can atomically swap it in without crossing filesystem
	// boundaries.
	backup, err := os.CreateTemp(filepath.Dir(awsConfigFilePath), ".aws-sso-manager-*.ini")
	if err != nil {
		return "", fmt.Errorf("creating backup temp file: %w", err)
	}

	replacement, err := os.ReadFile(tmpFile) // lint:allow_dynamic_filename
	if err != nil {
		return "", fmt.Errorf("reading replacement file: %w", err)
	}

	conf, err := os.Open(awsConfigFilePath)
	if err != nil {
		return "", fmt.Errorf(fmtOpeningAWSConfigFile, err)
	}

	defer func() {
		_ = conf.Close()   // lint:allow_unhandled
		_ = backup.Close() // lint:allow_unhandled
	}()

	confScanner := bufio.NewScanner(conf)
	inManagedBlock := false
	injectedInBlock := false

	for confScanner.Scan() {
		confLine := confScanner.Text()

		if strings.Contains(confLine, "aws-sso-manager: start "+profileName) {
			logger.DebugContext(context.Background(), ">| "+confLine)

			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", fmt.Errorf("writing start marker to backup: %w", err)
			}

			inManagedBlock = true
			injectedInBlock = false

			continue
		}

		if strings.Contains(confLine, "aws-sso-manager: end "+profileName) {
			logger.DebugContext(context.Background(), "<| "+confLine)

			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", fmt.Errorf("writing end marker to backup: %w", err)
			}

			inManagedBlock = false
			injectedInBlock = false

			continue
		}

		logger.DebugContext(context.Background(), " | "+confLine)

		if inManagedBlock {
			if !injectedInBlock {
				if _, err = backup.Write(replacement); err != nil {
					return "", fmt.Errorf("writing replacement to backup: %w", err)
				}

				injectedInBlock = true
			}
		} else {
			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", fmt.Errorf("writing line to backup: %w", err)
			}
		}
	}

	// Check for errors.
	if err := confScanner.Err(); err != nil {
		fmt.Println(err)
	}

	return backup.Name(), nil
}

// getAllMarkedProfiles returns the profile names for every managed-block start
// marker found in the AWS config file, regardless of whether a matching
// [sso-session] section exists. This is used by the validate command to detect
// orphaned markers.
func getAllMarkedProfiles() ([]string, error) {
	report, err := inspectManagedMarkers()
	if err != nil {
		return nil, fmt.Errorf(fmtInspectingManagedMarkers, err)
	}

	return report.profiles, nil
}

func getAllManagedSections() ([]string, error) {
	var ssoProfiles []string

	logger.DebugContext(context.Background(), "Reading from AWS config", logKeyFile, awsConfigFilePath)

	conf, err := os.Open(awsConfigFilePath)
	if err != nil {
		return ssoProfiles, fmt.Errorf(fmtOpeningAWSConfigFile, err)
	}

	defer func() {
		_ = conf.Close() // lint:allow_unhandled
	}()

	confScanner := bufio.NewScanner(conf)

	for confScanner.Scan() {
		confLine := strings.TrimSpace(confScanner.Text())
		prefix := "[sso-session "

		if strings.HasPrefix(confLine, prefix) {
			ssoProfiles = append(ssoProfiles, confLine[len(prefix):len(confLine)-1])
		}
	}

	if confScanner.Err() != nil {
		return ssoProfiles, fmt.Errorf("scanning AWS config: %w", confScanner.Err())
	}

	slices.Sort(ssoProfiles)

	return ssoProfiles, nil
}
