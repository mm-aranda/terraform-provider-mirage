package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/impersonate"
)

// DagGeneratorAPIClient handles the underlying communication (e.g., HTTP, auth).
type DagGeneratorAPIClient struct {
	BaseURL               string
	HTTPClient            *http.Client
	useServiceAccountAuth bool
	idTokenSource         oauth2.TokenSource
	authInitErr           error
}

func NewDagGeneratorAPIClient(baseURL string) *DagGeneratorAPIClient {
	return &DagGeneratorAPIClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

// NewDagGeneratorAPIClientWithAuth creates a client with optional service account auth.
func NewDagGeneratorAPIClientWithAuth(baseURL string, useServiceAccountAuth bool) *DagGeneratorAPIClient {
	client := &DagGeneratorAPIClient{
		BaseURL:               baseURL,
		HTTPClient:            &http.Client{},
		useServiceAccountAuth: useServiceAccountAuth,
	}

	if useServiceAccountAuth {
		ts, desc, err := newIDTokenSource(context.Background(), baseURL)
		if err != nil {
			client.authInitErr = err
		} else {
			fmt.Printf("[AUTH] Using %s\n", desc)
			client.idTokenSource = ts
		}
	}
	return client
}

// newIDTokenSource returns an OIDC ID token source for invoking the
// IAM-protected Cloud Run backend. Cloud Run rejects plain OAuth2 access
// tokens, so every path here must yield an ID token. In order:
//  1. GOOGLE_IMPERSONATE_SERVICE_ACCOUNT, if set (works for any caller that
//     has roles/iam.serviceAccountTokenCreator on that account).
//  2. idtoken.NewTokenSource — works when ADC is a service account.
//  3. User ADC: the OAuth refresh response carries an `id_token` alongside
//     the access token (same mechanism as `gcloud auth print-identity-token`);
//     Cloud Run accepts these user ID tokens even though their audience is
//     the gcloud client ID rather than the service URL.
func newIDTokenSource(ctx context.Context, audience string) (oauth2.TokenSource, string, error) {
	if sa := os.Getenv("GOOGLE_IMPERSONATE_SERVICE_ACCOUNT"); sa != "" {
		ts, err := impersonate.IDTokenSource(ctx, impersonate.IDTokenConfig{
			TargetPrincipal: sa,
			Audience:        serviceOrigin(audience),
			IncludeEmail:    true,
		})
		if err != nil {
			return nil, "", fmt.Errorf("impersonating %s (from GOOGLE_IMPERSONATE_SERVICE_ACCOUNT): %w", sa, err)
		}
		return ts, fmt.Sprintf("impersonated ID token (%s)", sa), nil
	}

	ts, err := idtoken.NewTokenSource(ctx, serviceOrigin(audience))
	if err == nil {
		return ts, "service account ID token", nil
	}
	if !strings.Contains(err.Error(), "unsupported credentials type") {
		return nil, "", fmt.Errorf("creating ID token source: %w", err)
	}

	// ADC is a user account: idtoken.NewTokenSource refuses those, but the
	// user refresh flow can still mint an ID token.
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, "", fmt.Errorf("no usable Google credentials: %w (run `gcloud auth application-default login`)", err)
	}
	return oauth2.ReuseTokenSource(nil, userIDTokenSource{base: creds.TokenSource}), "user-credential ID token", nil
}

// userIDTokenSource extracts the `id_token` field that Google's token
// endpoint returns next to the access token when refreshing user credentials.
type userIDTokenSource struct {
	base oauth2.TokenSource
}

func (s userIDTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	idt, _ := tok.Extra("id_token").(string)
	if idt == "" {
		return nil, fmt.Errorf("user credentials did not include an ID token, so the Cloud Run backend cannot be invoked: " +
			"re-run `gcloud auth application-default login`, or set GOOGLE_IMPERSONATE_SERVICE_ACCOUNT=<sa> to impersonate a service account")
	}
	return &oauth2.Token{AccessToken: idt, Expiry: tok.Expiry}, nil
}

// serviceOrigin trims the path from the backend URL: Cloud Run expects the
// ID token audience to be the bare service URL, not e.g. .../api/v1.
func serviceOrigin(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return rawURL
	}
	return u.Scheme + "://" + u.Host
}

// addAuthHeader adds the ID token if service account auth is enabled.
func (c *DagGeneratorAPIClient) addAuthHeader(ctx context.Context, req *http.Request) error {
	if !c.useServiceAccountAuth {
		return nil
	}
	if c.authInitErr != nil {
		return fmt.Errorf("authentication was not initialized: %w", c.authInitErr)
	}
	if c.idTokenSource == nil {
		return fmt.Errorf("authentication is enabled but no token source is available")
	}
	token, err := c.idTokenSource.Token()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	return nil
}

// DagGeneratorService handles the API calls for the dag_generator resource.
type DagGeneratorService struct {
	Client *DagGeneratorAPIClient
}

// GenerateResponse matches the JSON from the backend's /generate endpoint.
type GenerateResponse struct {
	Checksum   string `json:"checksum"`
	Generation string `json:"generation"`
}

// StatusResponse matches the JSON from the backend's /status endpoint.
type StatusResponse struct {
	Checksum   string `json:"checksum"`
	Generation string `json:"generation"`
}

// TemplateStatusResponse matches the JSON from the backend's /template-status endpoint.
type TemplateStatusResponse struct {
	Checksum     string `json:"checksum"`
	LastModified string `json:"last_modified"`
	Generation   string `json:"generation"`
	Exists       bool   `json:"exists"`
}

// Generate calls the backend to create or update a file.
func (s *DagGeneratorService) Generate(ctx context.Context, templatePath, templateContent, targetPath string, contextJSON string) (*GenerateResponse, error) {
	url := fmt.Sprintf("%s/generate", s.Client.BaseURL)

	payload := map[string]interface{}{
		"template_gcs_path": templatePath,
		"template_content":  templateContent,
		"target_gcs_path":   targetPath,
		"context_json":      contextJSON,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if err := s.Client.addAuthHeader(ctx, req); err != nil {
		return nil, err
	}

	resp, err := s.Client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, err
	}

	return &genResp, nil
}

// GetStatus retrieves the current checksum and generation for a file.
func (s *DagGeneratorService) GetStatus(ctx context.Context, path string) (*StatusResponse, error) {
	url := fmt.Sprintf("%s/status?target_gcs_path=%s", s.Client.BaseURL, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if err := s.Client.addAuthHeader(ctx, req); err != nil {
		return nil, err
	}

	resp, err := s.Client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file not found")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var statusResp StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, err
	}

	return &statusResp, nil
}

// GetTemplateStatus retrieves the current status of a template file.
func (s *DagGeneratorService) GetTemplateStatus(ctx context.Context, templatePath string) (*TemplateStatusResponse, error) {
	url := fmt.Sprintf("%s/template-status?template_gcs_path=%s", s.Client.BaseURL, templatePath)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if err := s.Client.addAuthHeader(ctx, req); err != nil {
		return nil, err
	}

	resp, err := s.Client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return &TemplateStatusResponse{Exists: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var templateStatusResp TemplateStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&templateStatusResp); err != nil {
		return nil, err
	}

	templateStatusResp.Exists = true
	return &templateStatusResp, nil
}

// Delete removes a file via the backend service.
func (s *DagGeneratorService) Delete(ctx context.Context, path string) error {
	url := fmt.Sprintf("%s/delete", s.Client.BaseURL)

	payload := map[string]interface{}{
		"target_gcs_path": path,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if err := s.Client.addAuthHeader(ctx, req); err != nil {
		return err
	}

	resp, err := s.Client.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
