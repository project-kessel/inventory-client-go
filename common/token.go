package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/patrickmn/go-cache"
	"google.golang.org/grpc"
)

const (
	defaultTokenHTTPTimeout      = 10 * time.Second
	defaultCacheCleanupInterval  = 5 * time.Minute
	tokenExpiryMargin            = 30 * time.Second
	client_credentials_granttype = "client_credentials"
)

type secureMetadataCreds map[string]string

func (c secureMetadataCreds) RequireTransportSecurity() bool { return true }
func (c secureMetadataCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return c, nil
}

// WithBearerToken returns a grpc.CallOption that adds a standard HTTP Bearer
// token to all requests sent from a client.
func WithBearerToken(token string) grpc.CallOption {
	return grpc.PerRPCCredentials(secureMetadataCreds{"Authorization": "Bearer " + token})
}

type insecureMetadataCreds map[string]string

func (c insecureMetadataCreds) RequireTransportSecurity() bool { return false }
func (c insecureMetadataCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return c, nil
}

// WithInsecureBearerToken returns a grpc.CallOption that adds a standard HTTP
// Bearer token to all requests sent from an insecure client.
//
// Must be used in conjunction with `insecure.NewCredentials()`.
func WithInsecureBearerToken(token string) grpc.CallOption {
	return grpc.PerRPCCredentials(insecureMetadataCreds{"Authorization": "Bearer " + token})
}

// NewTokenClient creates and returns a new TokenClient client.
func NewTokenClient(config *Config) *TokenClient {
	timeout := config.TokenHTTPTimeout
	if timeout == 0 {
		timeout = defaultTokenHTTPTimeout
	}

	return &TokenClient{
		clientId:       config.clientId,
		clientSecret:   config.clientSecret,
		url:            config.authServerTokenUrl,
		EnableOIDCAuth: config.EnableOIDCAuth,
		Insecure:       config.Insecure,
		cache:          cache.New(cache.NoExpiration, defaultCacheCleanupInterval),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type TokenClient struct {
	clientId       string
	clientSecret   string
	url            string
	EnableOIDCAuth bool
	Insecure       bool
	cache          *cache.Cache
	httpClient     *http.Client
}

func (a *TokenClient) GetCachedToken(tokenKey string) (string, error) {
	cachedToken, isCached := a.cache.Get(tokenKey)
	ct, _ := cachedToken.(string)
	if isCached {
		return ct, nil
	}
	return "", fmt.Errorf("failed to retrieve cached token")
}

func IsJWTTokenExpired(accessToken string) (bool, time.Time) {
	if token, _ := jwt.Parse(accessToken, nil); token != nil {
		tokenClaims := token.Claims.(jwt.MapClaims)
		if _, ok := tokenClaims["exp"]; ok {
			expTime := time.Unix(int64(tokenClaims["exp"].(float64)), 0)
			return time.Now().After(expTime), expTime
		}
	}
	return true, time.Time{}
}

func (a *TokenClient) GetToken() (*TokenResponse, error) {
	return a.GetTokenWithContext(context.Background())
}

// GetTokenWithContext retrieves an access token, using the cache when possible.
// The provided context controls cancellation and timeout of the SSO request.
func (a *TokenClient) GetTokenWithContext(ctx context.Context) (*TokenResponse, error) {
	cachedTokenKey := a.url + a.clientId
	cachedToken, _ := a.GetCachedToken(cachedTokenKey)
	isExpired, _ := IsJWTTokenExpired(cachedToken)
	if cachedToken != "" && !isExpired {
		return &TokenResponse{AccessToken: cachedToken}, nil
	}

	data := url.Values{}
	data.Set("client_id", a.clientId)
	data.Set("client_secret", a.clientSecret)
	data.Set("grant_type", client_credentials_granttype)
	req, err := http.NewRequestWithContext(ctx, "POST", a.url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, err
	}

	// Cache based on actual token lifetime from SSO, with a safety margin
	// to ensure we refresh before expiry. Skip caching for tokens that are
	// too short-lived to benefit from it.
	cacheDuration := time.Duration(tokenResponse.ExpiresIn) * time.Second
	if cacheDuration > tokenExpiryMargin {
		cacheDuration -= tokenExpiryMargin
		a.cache.Set(cachedTokenKey, tokenResponse.AccessToken, cacheDuration)
	}

	return &tokenResponse, nil
}
