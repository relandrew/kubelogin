package token

//go:generate sh -c "mockgen -destination mock_$GOPACKAGE/execCredentialPlugin.go github.com/Azure/kubelogin/pkg/token ExecCredentialPlugin"

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/golang-jwt/jwt/v4"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/groups"
	"github.com/microsoftgraph/msgraph-sdk-go/models"

	"github.com/Azure/go-autorest/autorest/adal"
	klog "k8s.io/klog/v2"
)

const (
	expirationDelta time.Duration = 60 * time.Second
)

type ExecCredentialPlugin interface {
	Do() error
}

type execCredentialPlugin struct {
	o                    *Options
	tokenCache           TokenCache
	execCredentialWriter ExecCredentialWriter
	provider             TokenProvider
	disableTokenCache    bool
	refresher            func(adal.OAuthConfig, string, string, string, *adal.Token) (TokenProvider, error)
}

func New(o *Options) (ExecCredentialPlugin, error) {

	klog.V(10).Info(o.ToString())
	provider, err := newTokenProvider(o)
	if err != nil {
		return nil, err
	}
	disableTokenCache := false
	if o.LoginMethod == ServicePrincipalLogin || o.LoginMethod == MSILogin || o.LoginMethod == WorkloadIdentityLogin || o.LoginMethod == AzureCLILogin {
		disableTokenCache = true
	}
	return &execCredentialPlugin{
		o:                    o,
		tokenCache:           &defaultTokenCache{},
		execCredentialWriter: &execCredentialWriter{},
		provider:             provider,
		refresher:            newManualToken,
		disableTokenCache:    disableTokenCache,
	}, nil
}

func (p *execCredentialPlugin) write(token adal.Token) error {
	p.logTokenInfo(token)

	return p.execCredentialWriter.Write(token, os.Stdout)
}

const logTokenLevel = 5

// Interceptor that debug-logs token expiry and groups
func (p *execCredentialPlugin) logTokenInfo(token adal.Token) {

	if !klog.V(logTokenLevel).Enabled() {
		return
	}

	klog.V(logTokenLevel).Infof("token expires in %v", token.Expires().Sub(time.Now()))

	err := p.logGroups(token)
	if err != nil {
		klog.V(logTokenLevel).Infof("warning: unable to log groups: %s", err.Error())
	}
}

func (p *execCredentialPlugin) logGroups(token adal.Token) error {
	type MyClaims struct {
		jwt.RegisteredClaims
		Groups []string `json:"groups,omitempty"`
	}

	var claims MyClaims
	_, _, err := jwt.NewParser().ParseUnverified(token.AccessToken, &claims)
	if err != nil {
		return fmt.Errorf("failed to parse tokekn as jwt: %w", err)
	}

	if claims.Groups == nil || len(claims.Groups) == 0 {
		return nil
	}

	groupNames := []string{}

	v := reflect.Indirect(reflect.ValueOf(p.provider)).FieldByName("tenantID")
	if v.Type().Kind() == reflect.String {
		tenantID := v.String()
		_, err := uuid.Parse(tenantID)
		tenantIDisGUID := err == nil

		credOptions := azidentity.DefaultAzureCredentialOptions{}
		if tenantIDisGUID {
			credOptions.TenantID = tenantID
		}

		cred, err := azidentity.NewDefaultAzureCredential(&credOptions)
		if err != nil {
			return fmt.Errorf("failed to get azure credential: %w", err)
		}
		gc, err := msgraphsdkgo.NewGraphServiceClientWithCredentials(cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create msgraph client: %w", err)
		}
		body := groups.NewGetByIdsPostRequestBody()
		body.SetIds(claims.Groups)
		r, err := gc.Groups().GetByIds().Post(context.TODO(), body, nil)
		if err != nil {
			return fmt.Errorf("failed to get identities of groups %v: %w", claims.Groups, err)
		}

		groups := r.GetValue()
		for _, g := range groups {
			g2 := g.(models.Groupable)
			groupNames = append(groupNames, *g2.GetDisplayName())
		}
	} else {
		groupNames = claims.Groups
	}

	sort.Strings(groupNames)
	if len(groupNames) > 0 {
		klog.V(logTokenLevel).Infof("Token group names:")
	}
	for _, g := range groupNames {
		klog.V(logTokenLevel).Infof("  - %v", g)
	}

	return nil
}

func (p *execCredentialPlugin) Do() error {
	var (
		token adal.Token
		err   error
	)
	if !p.disableTokenCache {
		// get token from cache
		token, err = p.tokenCache.Read(p.o.tokenCacheFile)
		if err != nil {
			return fmt.Errorf("unable to read from token cache: %s, err: %s", p.o.tokenCacheFile, err)
		}
	}

	// verify resource
	targetAudience := p.o.ServerID
	if p.o.IsLegacy {
		targetAudience = fmt.Sprintf("spn:%s", p.o.ServerID)
	}
	if token.Resource == targetAudience && !token.IsZero() {
		// if not expired, return
		if os.Getenv("KUBELOGIN_FORCE_REFRESH") == "" && !token.WillExpireIn(expirationDelta) {
			klog.V(10).Info("access token is still valid. will return")
			return p.write(token)
		}

		// if expired, try refresh when refresh token exists
		if token.RefreshToken != "" {
			tokenRefreshed := false
			klog.V(10).Info("getting refresher")
			oAuthConfig, err := getOAuthConfig(p.o.Environment, p.o.TenantID, p.o.IsLegacy)
			if err != nil {
				return fmt.Errorf("unable to get oAuthConfig: %s", err)
			}
			refresher, err := p.refresher(*oAuthConfig, p.o.ClientID, p.o.ServerID, p.o.TenantID, &token)
			if err != nil {
				return fmt.Errorf("failed to get refresher: %s", err)
			}
			klog.V(5).Info("refresh token")
			token, err := refresher.Token()
			// if refresh fails, we will login using token provider
			if err != nil {
				klog.V(5).Infof("refresh failed, will continue to login: %s", err)
			} else {
				tokenRefreshed = true
			}

			if tokenRefreshed {
				klog.V(10).Info("token refreshed")

				// if refresh succeeds, save tooken, and return
				if err := p.tokenCache.Write(p.o.tokenCacheFile, token); err != nil {
					return fmt.Errorf("failed to write to store: %s", err)
				}

				return p.write(token)
			}
		} else {
			klog.V(5).Info("there is no refresh token")
		}
	}

	klog.V(5).Info("acquire new token")
	// run the underlying provider
	token, err = p.provider.Token()
	if err != nil {
		return fmt.Errorf("failed to get token: %s", err)
	}

	if !p.disableTokenCache {
		// save token
		if err := p.tokenCache.Write(p.o.tokenCacheFile, token); err != nil {
			return fmt.Errorf("unable to write to token cache: %s, err: %s", p.o.tokenCacheFile, err)
		}
	}

	return p.write(token)
}
