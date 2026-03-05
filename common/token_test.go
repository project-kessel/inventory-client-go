package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func createTestJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": exp.Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign test JWT: %v", err)
	}
	return signed
}

func newTestTokenServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func tokenHandler(accessToken string, expiresIn int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			AccessToken: accessToken,
			ExpiresIn:   expiresIn,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func newTestClient(serverURL string) *TokenClient {
	config := NewConfig(
		WithAuthEnabled("test-client", "test-secret", serverURL),
	)
	return NewTokenClient(config)
}

func TestGetTokenWithContext_FetchesFromServer(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(10 * time.Minute))
	server := newTestTokenServer(tokenHandler(token, 600))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.AccessToken != token {
		t.Errorf("expected token %q, got %q", token, resp.AccessToken)
	}
}

func TestGetTokenWithContext_ReturnsCachedToken(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(10 * time.Minute))
	callCount := 0
	server := newTestTokenServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		tokenHandler(token, 600)(w, r)
	})
	defer server.Close()

	client := newTestClient(server.URL)

	// First call hits the server
	_, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("first call: expected no error, got %v", err)
	}

	// Second call should use cache
	resp, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("second call: expected no error, got %v", err)
	}
	if resp.AccessToken != token {
		t.Errorf("expected cached token %q, got %q", token, resp.AccessToken)
	}
	if callCount != 1 {
		t.Errorf("expected 1 server call, got %d", callCount)
	}
}

func TestGetTokenWithContext_RefetchesExpiredToken(t *testing.T) {
	expiredToken := createTestJWT(t, time.Now().Add(-1 * time.Minute))
	freshToken := createTestJWT(t, time.Now().Add(10 * time.Minute))

	callCount := 0
	server := newTestTokenServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			tokenHandler(expiredToken, 0)(w, r)
		} else {
			tokenHandler(freshToken, 600)(w, r)
		}
	})
	defer server.Close()

	client := newTestClient(server.URL)

	// First call gets the expired token
	_, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("first call: expected no error, got %v", err)
	}

	// Second call should re-fetch because the cached token is expired
	resp, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("second call: expected no error, got %v", err)
	}
	if resp.AccessToken != freshToken {
		t.Errorf("expected fresh token, got expired one")
	}
	if callCount != 2 {
		t.Errorf("expected 2 server calls, got %d", callCount)
	}
}

func TestGetTokenWithContext_ContextCancellation(t *testing.T) {
	server := newTestTokenServer(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow SSO server
		time.Sleep(1 * time.Second)
		tokenHandler(createTestJWT(t, time.Now().Add(10*time.Minute)), 600)(w, r)
	})
	defer server.Close()

	client := newTestClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetTokenWithContext(ctx)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestGetTokenWithContext_ServerError(t *testing.T) {
	server := newTestTokenServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := client.GetTokenWithContext(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	expected := "unexpected status code: 500"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestGetTokenWithContext_CacheDurationRespectsExpiresIn(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(10 * time.Minute))
	server := newTestTokenServer(tokenHandler(token, 600))
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the token is in the cache with a TTL based on expires_in
	cachedTokenKey := server.URL + "test-client"
	_, expiration, found := client.cache.GetWithExpiration(cachedTokenKey)
	if !found {
		t.Fatal("expected token to be cached")
	}

	// The expiration should be roughly expires_in (600s) minus the margin (30s) from now
	expectedExpiry := time.Now().Add(570 * time.Second)
	diff := expiration.Sub(expectedExpiry)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("cache expiry not based on expires_in: expected ~%v, got %v (diff: %v)",
			expectedExpiry, expiration, diff)
	}
}

func TestGetTokenWithContext_DoesNotCacheShortLivedTokens(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(30*time.Second))
	server := newTestTokenServer(tokenHandler(token, 30))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	cachedTokenKey := server.URL + "test-client"
	_, found := client.cache.Get(cachedTokenKey)
	if found {
		t.Error("expected short-lived token to not be cached")
	}
}

func TestGetToken_BackwardCompatible(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(10 * time.Minute))
	server := newTestTokenServer(tokenHandler(token, 600))
	defer server.Close()

	client := newTestClient(server.URL)

	// GetToken() should work the same as GetTokenWithContext(context.Background())
	resp, err := client.GetToken()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.AccessToken != token {
		t.Errorf("expected token %q, got %q", token, resp.AccessToken)
	}
}

func TestNewTokenClient_DefaultTimeout(t *testing.T) {
	config := NewConfig(
		WithAuthEnabled("id", "secret", "http://localhost"),
	)
	client := NewTokenClient(config)

	if client.httpClient.Timeout != defaultTokenHTTPTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultTokenHTTPTimeout, client.httpClient.Timeout)
	}
}

func TestNewTokenClient_CustomTimeout(t *testing.T) {
	config := NewConfig(
		WithAuthEnabled("id", "secret", "http://localhost"),
	)
	config.TokenHTTPTimeout = 3 * time.Second
	client := NewTokenClient(config)

	if client.httpClient.Timeout != 3*time.Second {
		t.Errorf("expected timeout 3s, got %v", client.httpClient.Timeout)
	}
}

func TestGetTokenWithContext_ReusesHTTPClient(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(10 * time.Minute))
	server := newTestTokenServer(tokenHandler(token, 600))
	defer server.Close()

	client := newTestClient(server.URL)
	originalHTTPClient := client.httpClient

	// Flush cache to force a second fetch
	client.cache.Flush()
	_, _ = client.GetTokenWithContext(context.Background())

	client.cache.Flush()
	_, _ = client.GetTokenWithContext(context.Background())

	if client.httpClient != originalHTTPClient {
		t.Error("expected http client to be reused across calls")
	}
}

func TestIsJWTTokenExpired_ExpiredToken(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(-1 * time.Minute))
	expired, _ := IsJWTTokenExpired(token)
	if !expired {
		t.Error("expected token to be expired")
	}
}

func TestIsJWTTokenExpired_ValidToken(t *testing.T) {
	token := createTestJWT(t, time.Now().Add(10 * time.Minute))
	expired, _ := IsJWTTokenExpired(token)
	if expired {
		t.Error("expected token to not be expired")
	}
}

func TestIsJWTTokenExpired_EmptyString(t *testing.T) {
	expired, _ := IsJWTTokenExpired("")
	if !expired {
		t.Error("expected empty string to be treated as expired")
	}
}

func TestGetTokenWithContext_SendsCorrectFormData(t *testing.T) {
	formReceived := false
	server := newTestTokenServer(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}

		if r.PostForm.Get("client_id") != "my-client" {
			t.Errorf("expected client_id=my-client, got %s", r.PostForm.Get("client_id"))
		}
		if r.PostForm.Get("client_secret") != "my-secret" {
			t.Errorf("expected client_secret=my-secret, got %s", r.PostForm.Get("client_secret"))
		}
		if r.PostForm.Get("grant_type") != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %s", r.PostForm.Get("grant_type"))
		}
		formReceived = true

		token := createTestJWT(t, time.Now().Add(10 * time.Minute))
		tokenHandler(token, 600)(w, r)
	})
	defer server.Close()

	config := NewConfig(
		WithAuthEnabled("my-client", "my-secret", server.URL),
	)
	client := NewTokenClient(config)

	_, err := client.GetTokenWithContext(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !formReceived {
		t.Error("server did not receive form data")
	}
}
