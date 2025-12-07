package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
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
)

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

// loadAWSConfig loads the AWS config file from disk and returns its sections.
func loadAWSConfig(awsConfigFilePath string) (ini.Sections, error) {
	sections, err := ini.OpenFile(awsConfigFilePath)
	if err != nil {
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

	err = os.MkdirAll(path.Join(userHomeDir, ".aws"), 0o0755)
	cobra.CheckErr(err)

	_, err = os.Stat(awsConfigFilePath)
	if os.IsNotExist(err) {
		awsConfigFile, err := os.Create(awsConfigFilePath)
		cobra.CheckErr(err)

		defer func() {
			err = awsConfigFile.Close()
			cobra.CheckErr(err)
		}()
	}

	return awsConfigFilePath
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
func getSDKConfig(sessionProfile ssoProfile) (aws.Config, error) {
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

	token := new(ssooidc.CreateTokenOutput)
	sleepPerCycle := 2 * time.Second
	startTime := time.Now()
	delta := time.Since(startTime)
	oidcClient := ssooidc.NewFromConfig(*input.sdkConfig)

	for delta < input.loginTimeout {
		// Keep trying until the user approves the request in the browser
		token, err = oidcClient.CreateToken(
			ctx, &ssooidc.CreateTokenInput{
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
