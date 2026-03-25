// Copyright 2025-2026, Northwood Labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	managedStartMarkerPrefix = "aws-sso-manager: start "
	managedEndMarkerPrefix   = "aws-sso-manager: end "
)

type managedMarkerReport struct {
	profiles    []string
	startCounts map[string]int
	endCounts   map[string]int
	issues      map[string][]string
}

func appendManagedMarkerIssue(issues map[string][]string, profile, issue string) {
	if slices.Contains(issues[profile], issue) {
		return
	}

	issues[profile] = append(issues[profile], issue)
}

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

func inspectManagedMarkers() (*managedMarkerReport, error) {
	f, err := os.Open(awsConfigFilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error closing file: %v\n", err)
		}
	}()

	report := &managedMarkerReport{
		startCounts: map[string]int{},
		endCounts:   map[string]int{},
		issues:      map[string][]string{},
	}
	seenProfiles := map[string]struct{}{}
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
					"overlapping managed block markers at line %d: found start marker for profile %q while profile %q block is still open",
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

		if profile, ok := parseManagedMarkerProfile(line, managedEndMarkerPrefix); ok {
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
					"overlapping managed block markers at line %d: found end marker for profile %q while profile %q block is still open",
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
	}

	if err := scanner.Err(); err != nil {
		return nil, err
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

func getProfileName(profileName, account, role string) string {
	var (
		order     = asvConfig.GetStringSlice(profileName + ".rename.pattern.order")
		delimiter = asvConfig.GetString(profileName + ".rename.pattern.delimiter")
		prefix    = asvConfig.GetString(profileName + ".rename.prefix")
		suffix    = asvConfig.GetString(profileName + ".rename.suffix")

		// accountGlobal = asvConfig.GetStringMapString(profileName + ".rename.accounts.global_regex_replace")
		accountSubstr = asvConfig.GetStringMapString(profileName + ".rename.accounts.substr_match_replace")
		// roleGlobal    = asvConfig.GetStringMapString(profileName + ".rename.roles.global_regex_replace")
		roleSubstr = asvConfig.GetStringMapString(profileName + ".rename.roles.substr_match_replace")
	)

	if len(order) == 0 {
		return buildDefaultProfileName(account, role)
	}

	if delimiter == "" {
		delimiter = "-"
	}

	orderCopy := []string{}

	for _, o := range order {
		switch strings.ToLower(o) {
		case "account":
			matched := false

			for match, replace := range accountSubstr {
				if strings.Contains(strings.ToLower(account), strings.ToLower(match)) {
					if replace != "" {
						orderCopy = append(orderCopy, strings.ToLower(replace))
					}

					matched = true
					continue
				}
			}

			if !matched {
				orderCopy = append(orderCopy, strings.ToLower(account))
			}
		case "role":
			matched := false

			for match, replace := range roleSubstr {
				if strings.Contains(strings.ToLower(role), strings.ToLower(match)) {
					if replace != "" {
						orderCopy = append(orderCopy, strings.ToLower(replace))
					}

					matched = true
					continue
				}
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
		}
	}

	generatedName := strings.TrimSpace(strings.Join(orderCopy, delimiter))
	if generatedName == "" {
		return buildDefaultProfileName(account, role)
	}

	return generatedName
}

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
// markers so that orphaned markers (e.g. after manual section-header deletion)
// are caught before a duplicate block is appended.
func markersExist(profileName string) (bool, error) {
	report, err := inspectManagedMarkers()
	if err != nil {
		return false, err
	}

	return report.startCounts[profileName] > 0, nil
}

// validateMarkers checks that the managed block for profileName is well-formed:
// exactly one start/end pair, with no mismatches or duplicates.
func validateMarkers(profileName string) error {
	report, err := inspectManagedMarkers()
	if err != nil {
		return err
	}

	var errs []error
	for _, issue := range report.issues[profileName] {
		errs = append(errs, errors.New(issue))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func validateManagedMarkers() error {
	report, err := inspectManagedMarkers()
	if err != nil {
		return err
	}

	var errs []error
	for _, profile := range report.profiles {
		for _, issue := range report.issues[profile] {
			errs = append(errs, errors.New(issue))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func getManagedSection(profileName string) (string, error) {
	if err := validateManagedMarkers(); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "aws-sso-manager-managed-*.ini")
	if err != nil {
		return "", err
	}

	logger.Debugf("Reading from %s...", awsConfigFilePath)

	f, err := os.Open(awsConfigFilePath)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = tmp.Close()
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	doCopy := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "aws-sso-manager: start "+profileName) {
			logger.Debugf(">| %s", line)

			doCopy = true
			continue
		} else if strings.Contains(line, "aws-sso-manager: end "+profileName) {
			logger.Debugf("<| %s", line)
			break
		} else {
			logger.Debugf(" | %s", line)
			if doCopy {
				_, err = tmp.WriteString(line + "\n")
				if err != nil {
					return "", err
				}
			}
		}
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}

	return tmp.Name(), nil
}

func setManagedSection(tmpFile, profileName string) (string, error) {
	// Create the backup in the same directory as the config file so that
	// os.Rename can atomically swap it in without crossing filesystem boundaries.
	backup, err := os.CreateTemp(filepath.Dir(awsConfigFilePath), ".aws-sso-manager-*.ini")
	if err != nil {
		return "", err
	}

	replacement, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", err
	}

	conf, err := os.Open(awsConfigFilePath)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = conf.Close()
		_ = backup.Close()
	}()

	confScanner := bufio.NewScanner(conf)
	inManagedBlock := false
	injectedInBlock := false

	for confScanner.Scan() {
		confLine := confScanner.Text()

		if strings.Contains(confLine, "aws-sso-manager: start "+profileName) {
			logger.Debugf(">| %s", confLine)

			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", err
			}

			inManagedBlock = true
			injectedInBlock = false
			continue
		} else if strings.Contains(confLine, "aws-sso-manager: end "+profileName) {
			logger.Debugf("<| %s", confLine)

			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", err
			}

			inManagedBlock = false
			injectedInBlock = false
			continue
		} else {
			logger.Debugf(" | %s", confLine)

			if inManagedBlock {
				if !injectedInBlock {
					if _, err = backup.Write(replacement); err != nil {
						return "", err
					}

					injectedInBlock = true
				}
			} else {
				_, err = backup.WriteString(confLine + "\n")
				if err != nil {
					return "", err
				}
			}
		}
	}

	// Check for errors
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
		return nil, err
	}

	return report.profiles, nil
}

func getAllManagedSections() ([]string, error) {
	var ssoProfiles []string

	logger.Debugf("Reading from %s...", awsConfigFilePath)

	conf, err := os.Open(awsConfigFilePath)
	if err != nil {
		return ssoProfiles, err
	}

	defer func() {
		_ = conf.Close()
	}()

	confScanner := bufio.NewScanner(conf)

	for confScanner.Scan() {
		confLine := strings.TrimSpace(confScanner.Text())
		prefix := "[sso-session "

		if strings.HasPrefix(confLine, prefix) {
			ssoProfiles = append(ssoProfiles, confLine[len(prefix):len(confLine)-1])
		}
	}

	slices.Sort(ssoProfiles)

	return ssoProfiles, nil
}

// func truncate(filename string, perm os.FileMode) error {
// 	logger.Debugf("Truncating %s...", filename)

// 	f, err := os.OpenFile(filename, os.O_TRUNC, perm)
// 	if err != nil {
// 		return fmt.Errorf("could not open file %q for truncation: %w", filename, err)
// 	}

// 	if err = f.Close(); err != nil {
// 		return fmt.Errorf("could not close file handler for %q after truncation: %w", filename, err)
// 	}

// 	return nil
// }
