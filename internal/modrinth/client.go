package modrinth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)


const (
	baseAPI = "https://api.modrinth.com/v2"
	updateURL = baseAPI + "/version_files/update"
	versionsFilesURL = baseAPI + "/version_files"
)

type UpdateRequest struct {
	Hashes []string `json:"hashes"`
	Algorithm string   `json:"algorithm"`
	Loaders []string   `json:"loaders"`
	GameVersions []string `json:"game_versions"`
}

type ModrinthFile struct {
	Hashes map[string]string `json:"hashes"`
	URL string `json:"url"`
	Filename string `json:"filename"`
	Primary bool `json:"primary"`
}

type Version struct {
	ID string `json:"id"`
	ProjectID string `json:"project_id"`
	VersionType string `json:"version_type"`
	VersionNumber string `json:"version_number"`
	GameVersions []string `json:"game_versions"`
	Loaders []string `json:"loaders"`
	Files []ModrinthFile `json:"files"`
}

type Client struct {
	httpClient *http.Client
	userAgent string
}

// Initialise a new Modrinth API client
func NewClient(githubUser, projectName, version string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		// Modrinth requires a user agent for API requests, so we include the GitHub repo and version for transparency
		userAgent: fmt.Sprintf("%s/%s (%s)", githubUser, projectName, version),
	}
}

func (c *Client) do(ctx context.Context, method string, url string, body interface{}, target interface{})  error {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) CheckForUpdates(ctx context.Context, hashes []string, mcVersion, loader string) (map[string]Version, error) {
	if len(hashes) == 0 {
		return nil, nil // No mods to check, return empty result
	}

	// Build the request payload
	reqBody := UpdateRequest{
		Hashes: hashes,
		Algorithm: "sha1",
		Loaders: []string{loader},
		GameVersions: []string{mcVersion},
	}

	// err := c.do(context.Background(), "POST", apiUrl, reqBody, &result)
	// if err != nil {
	// 	return nil, fmt.Errorf("API request failed: %w", err)
	// }

	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	return nil, fmt.Errorf("unexpected API response status: %s", resp.Status)
	// }

	var result map[string]Version
	err := c.do(ctx, "POST", updateURL, reqBody, &result)
	return result, err
}

// GetVersionFromHashes identifies current mods
func (c *Client) CheckVersionsFromHashes(ctx context.Context, hashes []string) (map[string]Version, error) {
	if len(hashes) == 0 {
		return nil, nil // No mods to check, return empty result
	}
	
	// Build the request payload
	payload := struct {
		Hashes []string `json:"hashes"`
		Algorithm string   `json:"algorithm"`
	}{
		Hashes: hashes,
		Algorithm: "sha1",
	}

	// resp, err := c.MakeRequest(payload, "POST", "https://api.modrinth.com/v2/version_files")
	// if err != nil {
	// 	return nil, fmt.Errorf("API request failed: %w", err)
	// }

	// defer resp.Body.Close()
	
	// if resp.StatusCode != http.StatusOK {
	// 	return nil, fmt.Errorf("unexpected API response status: %s", resp.Status)
	// }

	var result map[string]Version
	err := c.do(ctx, "POST", versionsFilesURL, payload, &result)

	return result, err

}