// Copyright 2025, Northwood Labs
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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	charmlog "github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/northwood-labs/aws-config-parser/ini"
)

type (
	ssoProfile struct {
		Name     string
		StartURL string
		Region   string
		Scopes   string
	}

	customerAuthInput struct {
		ctx            context.Context
		sdkConfig      *aws.Config
		registerClient *ssooidc.RegisterClientOutput
		deviceAuth     *ssooidc.StartDeviceAuthorizationOutput
		sessionProfile ssoProfile
		loginTimeout   time.Duration
	}

	cacheFileData struct {
		ExpiresAt             time.Time `json:"expiresAt"`
		RegistrationExpiresAt time.Time `json:"registrationExpiresAt"`
		StartUrl              string    `json:"startUrl"`
		Region                string    `json:"region"`
		AccessToken           string    `json:"accessToken"`
		ClientId              string    `json:"clientId"`
		ClientSecret          string    `json:"clientSecret"`
	}

	listAWSAccountsInput struct {
		Cmd           *cobra.Command
		SDKConfig     *aws.Config
		Cache         *cacheFileData
		Logger        *charmlog.Logger
		ProfileName   string
		AccountFilter string
		RoleFilter    string
	}

	listAWSAccountsCacheData struct {
		CachedAt  time.Time    `json:"cached_at,omitempty"`
		ExpiresAt time.Time    `json:"expires_at,omitempty"`
		Accounts  listAccounts `json:"accounts"`
	}
)

var listAWSAccountsFetcher = fetchListAWSAccountsFromSSO

func (c *cacheFileData) save(cacheFilePath string) error {
	marshaledJson, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("could not marshal data into JSON: %w ", err)
	}

	dir, _ := path.Split(cacheFilePath)

	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("could not create a cache directory at %s: %w ", dir, err)
	}

	err = os.WriteFile(cacheFilePath, marshaledJson, 0o666)
	if err != nil {
		return fmt.Errorf("could not create a cache file at %s: %w ", cacheFilePath, err)
	}

	return nil
}

func (c *cacheFileData) read(cacheFilePath string) (*cacheFileData, error) {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read cache file at %s: %w ", cacheFilePath, err)
	}

	var cache cacheFileData
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal cache file data: %w ", err)
	}

	if time.Now().After(cache.ExpiresAt) {
		return nil, errors.New("the SSO access token has expired")
	}

	return &cache, nil
}

func ensureAWSManagerCacheDir() (string, error) {
	cacheDir := awsManagerCacheDir
	if cacheDir == "" {
		homeDir := userHomeDir
		if homeDir == "" {
			var err error
			homeDir, err = os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("could not determine user home directory: %w", err)
			}
		}

		cacheDir = filepath.Join(homeDir, ".config", "aws-sso-manager", "cache")
	}

	if err := os.MkdirAll(cacheDir, 0o0755); err != nil {
		return "", fmt.Errorf("could not create AWS manager cache directory: %w", err)
	}

	return cacheDir, nil
}

func (input listAWSAccountsInput) getLogger() *charmlog.Logger {
	if input.Logger != nil {
		return input.Logger
	}

	return logger
}

func (input listAWSAccountsInput) cacheFilePath() string {
	cacheKey := strings.Join([]string{
		"listAWSAccounts.v1",
		input.ProfileName,
		input.AccountFilter,
		input.RoleFilter,
	}, "\x00")
	hash := sha256.Sum256([]byte(cacheKey))
	cacheDir, err := ensureAWSManagerCacheDir()
	if err != nil {
		input.getLogger().Error("Failed to ensure AWS accounts cache directory", "error", err)
		return ""
	}

	return filepath.Join(cacheDir, "accounts-"+hex.EncodeToString(hash[:])+".json")
}

func readListAWSAccountsCache(cacheFilePath string) (listAccounts, bool, error) {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return listAccounts{}, false, nil
		}

		return listAccounts{}, false, fmt.Errorf("could not read accounts cache file: %w", err)
	}

	var cached listAWSAccountsCacheData
	if err := json.Unmarshal(data, &cached); err != nil {
		return listAccounts{}, false, fmt.Errorf("could not unmarshal accounts cache file: %w", err)
	}

	cacheTTL := cacheDuration
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}

	var expiresAt time.Time
	if !cached.CachedAt.IsZero() {
		expiresAt = cached.CachedAt.Add(cacheTTL)
	} else {
		expiresAt = cached.ExpiresAt
	}

	if expiresAt.IsZero() || time.Now().After(expiresAt) {
		if err := os.Remove(cacheFilePath); err != nil && !os.IsNotExist(err) {
			return listAccounts{}, false, fmt.Errorf("could not remove expired accounts cache file: %w", err)
		}

		return listAccounts{}, false, nil
	}

	return cached.Accounts, true, nil
}

func writeListAWSAccountsCache(cacheFilePath string, accounts listAccounts) error {
	cacheData := listAWSAccountsCacheData{
		CachedAt: time.Now().UTC(),
		Accounts: accounts,
	}

	data, err := json.Marshal(cacheData)
	if err != nil {
		return fmt.Errorf("could not marshal accounts cache file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cacheFilePath), 0o0755); err != nil {
		return fmt.Errorf("could not create accounts cache directory: %w", err)
	}

	tmpPath := cacheFilePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o0600); err != nil {
		return fmt.Errorf("could not write temporary accounts cache file: %w", err)
	}

	if err := os.Rename(tmpPath, cacheFilePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("could not replace accounts cache file: %w", err)
	}

	return nil
}

// loadAWSConfig loads the AWS config file from disk and returns its sections.
func loadAWSConfig(awsConfigFilePath string) (ini.Sections, error) {
	logger.Debug("Opening AWS config", "config", awsConfigFilePath)

	sections, err := ini.OpenFile(awsConfigFilePath)
	if err != nil {
		logger.Debug("Creating a fresh AWS config", "config", awsConfigFilePath)

		awsConfigFilePath = createAWSConfigFile()

		sections, err = ini.OpenFile(awsConfigFilePath)
		if err != nil {
			return ini.NewSections(), fmt.Errorf("failed to open AWS config file: %w", err)
		}
	}

	return sections, nil
}

// createAWSConfigFile creates the config file if it does not exist.
func createAWSConfigFile() string {
	userHomeDir, err := os.UserHomeDir()
	cobra.CheckErr(err)

	logger.Debug("User home directory", "home", userHomeDir)

	err = os.MkdirAll(path.Join(userHomeDir, ".aws"), 0o0755)
	cobra.CheckErr(err)

	awsConfigFile, err := os.OpenFile(awsConfigFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o0644)
	if err == nil {
		defer func() {
			err = awsConfigFile.Close()
			cobra.CheckErr(err)
		}()
	} else if !os.IsExist(err) {
		cobra.CheckErr(err)
	}

	return awsConfigFilePath
}

// generateSingleAWSConfig generates the AWS config file content from the given sections.
func generateSingleAWSConfig(section ini.Section) string {
	var out strings.Builder

	fmt.Fprintf(&out, "[%s]\n", section.Name)

	for _, key := range section.List() {
		fmt.Fprintf(&out, "%s = %s\n", key, section.String(key))
	}

	fmt.Fprintln(&out, "")

	return out.String()
}

// generateAWSConfig generates the AWS config file content from the given sections.
func generateAWSConfig(sections ini.Sections) string {
	var out strings.Builder

	for _, sectionName := range sections.List() {
		fmt.Fprintf(&out, "[%s]\n", sectionName)

		section, ok := sections.GetSection(sectionName)
		if !ok {
			continue
		}

		for _, key := range section.List() {
			fmt.Fprintf(&out, "%s = %s\n", key, section.String(key))
		}

		fmt.Fprintln(&out, "")
	}

	return out.String()
}

// getAWSConfig generates an SDK config from the SSO session profile.
func getSDKConfig(ctx context.Context, sessionProfile ssoProfile) (aws.Config, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	awsConfig, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(sessionProfile.Region),
		config.WithSharedConfigFiles([]string{awsConfigFilePath}),
	)
	if err != nil {
		var (
			credentialsErr    config.CredentialRequiresARNError
			assumeRoleErr     config.SharedConfigAssumeRoleError
			configLoadErr     config.SharedConfigLoadError
			missingProfileErr config.SharedConfigProfileNotExistError
		)

		if errors.As(err, &credentialsErr) {
			return aws.Config{}, fmt.Errorf("credential requires arn error: %w", err)
		} else if errors.As(err, &assumeRoleErr) {
			return aws.Config{}, fmt.Errorf("shared config assume role error: %w", err)
		} else if errors.As(err, &configLoadErr) {
			return aws.Config{}, fmt.Errorf("shared config load error: %w", err)
		} else if errors.As(err, &missingProfileErr) {
			return aws.Config{}, fmt.Errorf("missing shared config profile error: %w", err)
		}

		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return awsConfig, nil
}

// getSsoSession retrieves the SSO session profile from the AWS config file.
func getSsoSession(profileName string) (ssoProfile, error) {
	sections, err := loadAWSConfig(awsConfigFilePath)
	if err != nil {
		return ssoProfile{}, fmt.Errorf("failed to load AWS config file: %w", err)
	}

	sessionName := fmt.Sprintf("sso-session %s", profileName)

	section, ok := sections.GetSection(sessionName)
	if !ok {
		return ssoProfile{}, fmt.Errorf(
			"the profile [%s] does not exist in the AWS config file",
			sessionName,
		)
	}

	profile := ssoProfile{
		Name:     profileName,
		StartURL: section.String("sso_start_url"),
		Region:   section.String("sso_region"),
		Scopes:   section.String("sso_registration_scopes"),
	}

	return profile, nil
}

func getCacheFilePath(sessionProfile *ssoProfile) (string, error) {
	var (
		key           string
		cacheFilePath string
		err           error
	)

	switch sessionProfile.Name {
	case "":
		key = sessionProfile.StartURL
	default:
		key = sessionProfile.Name
	}

	cacheFilePath, err = ssocreds.StandardCachedTokenFilepath(key)
	if err != nil {
		return "", fmt.Errorf("failed to get SSO cache file path: %w", err)
	}

	logger.Debug("Cached data file", "file", cacheFilePath)

	return cacheFilePath, nil
}

// authenticateSSOProfile performs the SSO authentication flow for the given SSO session profile.
func authenticateSSOProfile(
	ctx context.Context,
	sdkConfig *aws.Config,
	sessionProfile ssoProfile,
) (string, *ssooidc.RegisterClientOutput, *ssooidc.StartDeviceAuthorizationOutput, error) {
	// Get current OS user.
	currentUser, err := user.Current()
	if err != nil {
		return "", &ssooidc.RegisterClientOutput{}, &ssooidc.StartDeviceAuthorizationOutput{}, fmt.Errorf(
			"failed to lookup current OS user: %w",
			err,
		)
	}

	logger.Debug("Current OS user", "user", currentUser.Username)

	clientName := currentUser.Username + "-" + sessionProfile.Name + "-" + sessionProfile.Region
	oidcClient := ssooidc.NewFromConfig(*sdkConfig)

	registerClient, err := oidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(clientName),
		ClientType: aws.String("public"),
		Scopes:     []string{sessionProfile.Scopes},
	})
	if err != nil {
		return "", &ssooidc.RegisterClientOutput{}, &ssooidc.StartDeviceAuthorizationOutput{}, fmt.Errorf(
			"failed to register a public client: %w",
			err,
		)
	}

	deviceAuth, err := oidcClient.StartDeviceAuthorization(
		ctx,
		&ssooidc.StartDeviceAuthorizationInput{
			ClientId:     registerClient.ClientId,
			ClientSecret: registerClient.ClientSecret,
			StartUrl:     &sessionProfile.StartURL,
		},
	)
	if err != nil {
		return "", &ssooidc.RegisterClientOutput{}, &ssooidc.StartDeviceAuthorizationOutput{}, fmt.Errorf(
			"failed to start device authorization: %w",
			err,
		)
	}

	authUrl := aws.ToString(deviceAuth.VerificationUriComplete)

	return authUrl, registerClient, deviceAuth, nil
}

// waitForCustomerToAuthenticate waits for the customer to complete the SSO authentication flow.
func waitForCustomerToAuthenticate(input customerAuthInput) (cacheFileData, error) {
	var err error

	if input.ctx == nil {
		input.ctx = context.Background()
	}

	token := new(ssooidc.CreateTokenOutput)
	sleepPerCycle := 2 * time.Second
	startTime := time.Now()
	delta := time.Since(startTime)
	oidcClient := ssooidc.NewFromConfig(*input.sdkConfig)

	for delta < input.loginTimeout {
		// Keep trying until the user approves the request in the browser
		token, err = oidcClient.CreateToken(
			input.ctx, &ssooidc.CreateTokenInput{
				ClientId:     input.registerClient.ClientId,
				ClientSecret: input.registerClient.ClientSecret,
				DeviceCode:   input.deviceAuth.DeviceCode,
				GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
			},
		)
		if err == nil {
			break
		}

		accessDeniedErr := new(types.AccessDeniedException)
		authPendingErr := new(types.AuthorizationPendingException)
		expiredTokenErr := new(types.ExpiredTokenException)
		internalServerErr := new(types.InternalServerException)
		invalidClientErr := new(types.InvalidClientException)
		invalidClientMetaErr := new(types.InvalidClientMetadataException)
		invalidGrantErr := new(types.InvalidGrantException)
		invalidRedirectUriErr := new(types.InvalidRedirectUriException)
		invalidRequestErr := new(types.InvalidRequestException)
		invalidRegionErr := new(types.InvalidRequestRegionException)
		invalidScopeErr := new(types.InvalidScopeException)
		slowDownErr := new(types.SlowDownException)
		unauthClientErr := new(types.UnauthorizedClientException)
		unsupportedGrantErr := new(types.UnsupportedGrantTypeException)

		if errors.As(err, &authPendingErr) {
			time.Sleep(sleepPerCycle)
			delta = time.Since(startTime)
			continue
		} else if errors.As(err, &accessDeniedErr) {
			return cacheFileData{}, fmt.Errorf("authentication denied: %w", err)
		} else if errors.As(err, &expiredTokenErr) {
			return cacheFileData{}, fmt.Errorf("token expired; need to reauthenticate: %w", err)
		} else if errors.As(err, &internalServerErr) {
			return cacheFileData{}, fmt.Errorf("internal server error: %w", err)
		} else if errors.As(err, &invalidClientErr) {
			return cacheFileData{}, fmt.Errorf("invalid client error: %w", err)
		} else if errors.As(err, &invalidClientMetaErr) {
			return cacheFileData{}, fmt.Errorf("invalid client metadata error: %w", err)
		} else if errors.As(err, &invalidGrantErr) {
			return cacheFileData{}, fmt.Errorf("invalid grant error: %w", err)
		} else if errors.As(err, &invalidRedirectUriErr) {
			return cacheFileData{}, fmt.Errorf("invalid redirect URL error: %w", err)
		} else if errors.As(err, &invalidRequestErr) {
			return cacheFileData{}, fmt.Errorf("invalid request error: %w", err)
		} else if errors.As(err, &invalidRegionErr) {
			return cacheFileData{}, fmt.Errorf("invalid region error: %w", err)
		} else if errors.As(err, &invalidScopeErr) {
			return cacheFileData{}, fmt.Errorf("invalid scope error: %w", err)
		} else if errors.As(err, &slowDownErr) {
			return cacheFileData{}, fmt.Errorf("too many requests: %w", err)
		} else if errors.As(err, &unauthClientErr) {
			return cacheFileData{}, fmt.Errorf("unauthorized client: %w", err)
		} else if errors.As(err, &unsupportedGrantErr) {
			return cacheFileData{}, fmt.Errorf("unauthorized grant type: %w", err)
		} else {
			return cacheFileData{}, fmt.Errorf("SSO workflow error: %w", err)
		}
	}

	// Checks to see if there is a valid token after the login timeout ends
	if err != nil || token.AccessToken == nil {
		return cacheFileData{}, fmt.Errorf("failed to create token: %w", err)
	}

	cacheFile := cacheFileData{
		StartUrl:              input.sessionProfile.StartURL,
		Region:                input.sessionProfile.Region,
		AccessToken:           *token.AccessToken,
		ExpiresAt:             time.Unix(time.Now().Unix()+int64(token.ExpiresIn), 0).UTC(),
		ClientSecret:          *input.registerClient.ClientSecret,
		ClientId:              *input.registerClient.ClientId,
		RegistrationExpiresAt: time.Unix(input.registerClient.ClientSecretExpiresAt, 0).UTC(),
	}

	return cacheFile, nil
}

func listAWSAccounts(input listAWSAccountsInput) (listAccounts, error) {
	cacheFilePath := input.cacheFilePath()
	inputLogger := input.getLogger()

	if cacheFilePath != "" {
		inputLogger.Debug("Checking AWS accounts cache", "file", cacheFilePath)

		cachedAccounts, ok, err := readListAWSAccountsCache(cacheFilePath)
		if err != nil {
			inputLogger.Error("Failed to read AWS accounts cache", "file", cacheFilePath, "error", err)
		} else if ok {
			inputLogger.Debug("Using cached AWS accounts", "file", cacheFilePath)
			return cachedAccounts, nil
		}
	}

	accounts, err := listAWSAccountsFetcher(input)
	if err != nil {
		return accounts, err
	}

	if cacheFilePath != "" {
		if err := writeListAWSAccountsCache(cacheFilePath, accounts); err != nil {
			inputLogger.Error("Failed to write AWS accounts cache", "file", cacheFilePath, "error", err)
		} else {
			inputLogger.Debug("Wrote AWS accounts cache", "file", cacheFilePath)
		}
	}

	return accounts, nil
}

func fetchListAWSAccountsFromSSO(input listAWSAccountsInput) (listAccounts, error) {
	var accts listAccounts
	if input.Cmd == nil {
		return accts, errors.New("command is required")
	}

	if input.SDKConfig == nil {
		return accts, errors.New("AWS SDK config is required")
	}

	if input.Cache == nil {
		return accts, errors.New("SSO cache data is required")
	}

	ssoClient := sso.NewFromConfig(*input.SDKConfig)

	paginator := sso.NewListAccountsPaginator(ssoClient, &sso.ListAccountsInput{
		AccessToken: &input.Cache.AccessToken,
		MaxResults:  aws.Int32(100),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(input.Cmd.Context())
		if err != nil {
			return accts, fmt.Errorf("could not list SSO accounts: %w", err)
		}

		for _, account := range output.AccountList {
			if input.AccountFilter != "" && !strings.Contains(
				strings.ToLower(aws.ToString(account.AccountName)),
				strings.ToLower(input.AccountFilter),
			) {
				continue
			}

			singleAccount := listAccount{
				ID:    aws.ToString(account.AccountId),
				Name:  aws.ToString(account.AccountName),
				Email: aws.ToString(account.EmailAddress),
			}

			rolePaginator := sso.NewListAccountRolesPaginator(ssoClient, &sso.ListAccountRolesInput{
				AccessToken: &input.Cache.AccessToken,
				AccountId:   account.AccountId,
				MaxResults:  aws.Int32(100),
			})

			for rolePaginator.HasMorePages() {
				roleOutput, err := rolePaginator.NextPage(input.Cmd.Context())
				if err != nil {
					return accts, fmt.Errorf(
						"could not list roles for account %s: %w",
						aws.ToString(account.AccountId),
						err,
					)
				}

				for _, role := range roleOutput.RoleList {
					if input.RoleFilter != "" && !strings.Contains(
						strings.ToLower(aws.ToString(role.RoleName)),
						strings.ToLower(input.RoleFilter),
					) {
						continue
					}

					singleAccount.Roles = append(singleAccount.Roles, listRole{
						AccountID: aws.ToString(account.AccountId),
						Name:      aws.ToString(role.RoleName),
						Profile: getProfileName(
							input.ProfileName,
							aws.ToString(account.AccountName),
							aws.ToString(role.RoleName),
						),
					})
				}
			}

			// Sort roles by name
			sort.SliceStable(singleAccount.Roles, func(i, j int) bool {
				return strings.ToLower(
					singleAccount.Roles[i].Name,
				) < strings.ToLower(
					singleAccount.Roles[j].Name,
				)
			})

			accts.Accounts = append(accts.Accounts, singleAccount)
		}
	}

	// Sort accounts by name
	sort.SliceStable(accts.Accounts, func(i, j int) bool {
		return strings.ToLower(accts.Accounts[i].Name) < strings.ToLower(accts.Accounts[j].Name)
	})

	return accts, nil
}
