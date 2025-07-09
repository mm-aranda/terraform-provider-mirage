package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DagGeneratorAPIClient handles the underlying communication (e.g., HTTP, auth).
type DagGeneratorAPIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewDagGeneratorAPIClient(baseURL string) *DagGeneratorAPIClient {
	return &DagGeneratorAPIClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
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
