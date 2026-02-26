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
	"fmt"
	"os"
	"slices"
	"strings"
)

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

	return strings.Join(orderCopy, delimiter)
}

func getManagedSection(profileName string) (string, error) {
	tmp, err := os.CreateTemp("", "aws-sso-vault-managed-*.ini")
	if err != nil {
		return "", err
	}

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

		if strings.Contains(line, "aws-sso-vault: start "+profileName) {
			doCopy = true
			continue
		} else if strings.Contains(line, "aws-sso-vault: end "+profileName) {
			break
		} else {
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
	backup, err := os.CreateTemp("", "aws-sso-vault-config-*.ini")
	if err != nil {
		return "", err
	}

	tmp, err := os.Open(tmpFile)
	if err != nil {
		return "", err
	}

	conf, err := os.Open(awsConfigFilePath)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = tmp.Close()
		_ = conf.Close()
		_ = backup.Close()
	}()

	tmpScanner := bufio.NewScanner(tmp)
	confScanner := bufio.NewScanner(conf)
	doInject := false

	for confScanner.Scan() {
		confLine := confScanner.Text()

		if strings.Contains(confLine, "aws-sso-vault: start "+profileName) {
			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", err
			}

			doInject = true
			continue
		} else if strings.Contains(confLine, "aws-sso-vault: end "+profileName) {
			_, err = backup.WriteString(confLine + "\n")
			if err != nil {
				return "", err
			}

			doInject = false
			continue
		} else {
			if doInject {
				for tmpScanner.Scan() {
					tmpLine := tmpScanner.Text()

					_, err = backup.WriteString(tmpLine + "\n")
					if err != nil {
						return "", err
					}
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
	if err = tmpScanner.Err(); err != nil {
		fmt.Println(err)
	}

	return backup.Name(), nil
}

func getAllManagedSections() ([]string, error) {
	var ssoProfiles []string

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

func truncate(filename string, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("could not open file %q for truncation: %w", filename, err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("could not close file handler for %q after truncation: %w", filename, err)
	}

	return nil
}
