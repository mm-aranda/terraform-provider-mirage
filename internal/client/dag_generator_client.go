package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

// DagGeneratorAPIClient handles the underlying communication (e.g., HTTP, auth).
type DagGeneratorAPIClient struct {
	BaseURL               string
	HTTPClient            *http.Client
	useServiceAccountAuth bool
	idTokenSource         oauth2.TokenSource
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
		// Try to create ID token source first
		ts, err := idtoken.NewTokenSource(context.Background(), baseURL)
		if err != nil {
			// Check if this is the "unsupported credentials type" error
			if strings.Contains(err.Error(), "unsupported credentials type") {
				fmt.Printf("[AUTH] Using OAuth2 access token fallback (user credentials detected)\n")
				// Try to get a regular OAuth2 token source as fallback
				ctx := context.Background()
				creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
				if err == nil {
					client.idTokenSource = creds.TokenSource
				}
			} else {
				fmt.Printf("[AUTH] Failed to initialize authentication: %v\n", err)
			}
		} else {
			fmt.Printf("[AUTH] Using ID token authentication\n")
			client.idTokenSource = ts
		}
	}
	return client
}

// addAuthHeader adds the ID token if service account auth is enabled.
func (c *DagGeneratorAPIClient) addAuthHeader(ctx context.Context, req *http.Request) error {
	if c.useServiceAccountAuth && c.idTokenSource != nil {
		token, err := c.idTokenSource.Token()
		if err != nil {
			return err
		}

		authHeader := "Bearer " + token.AccessToken
		req.Header.Set("Authorization", authHeader)
	}
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
	Checksum         string `json:"checksum"`
	LastModified     string `json:"last_modified"`
	Generation       string `json:"generation"`
	Exists           bool   `json:"exists"`
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
