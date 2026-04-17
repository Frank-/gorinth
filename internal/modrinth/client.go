package modrinth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseAPI          = "https://api.modrinth.com/v2"
	updateURL        = baseAPI + "/version_files/update"
	versionsFilesURL = baseAPI + "/version_files"
	projectsURL      = baseAPI + "/projects?ids=%s"
)

type UpdateRequest struct {
	Hashes       []string `json:"hashes"`
	Algorithm    string   `json:"algorithm"`
	Loaders      []string `json:"loaders"`
	GameVersions []string `json:"game_versions"`
}

type VersionRequest struct {
	Hashes    []string `json:"hashes"`
	Algorithm string   `json:"algorithm"`
}

type ModrinthFile struct {
	Hashes   map[string]string `json:"hashes"`
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
}

type ModrinthProject struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Slug        string   `json:"slug"`
	GameID      string   `json:"game_id"`
	Loader      []string `json:"loader"`
}

type Version struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	VersionType   string         `json:"version_type"`
	VersionNumber string         `json:"version_number"`
	GameVersions  []string       `json:"game_versions"`
	Loaders       []string       `json:"loaders"`
	Files         []ModrinthFile `json:"files"`
}

type Client struct {
	httpClient *http.Client
	userAgent  string
}

// Initialise a new Modrinth API client
func NewClient(githubUser, projectName, version string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		// Modrinth requires a user agent for API requests, so we include the GitHub repo and version for transparency
		userAgent: fmt.Sprintf("%s/%s (%s)", githubUser, projectName, version),
	}
}

func (c *Client) do(ctx context.Context, method string, url string, body interface{}, target interface{}) error {
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
		return nil, nil
	}

	// Build the request
	reqBody := UpdateRequest{
		Hashes:       hashes,
		Algorithm:    "sha1",
		Loaders:      []string{loader},
		GameVersions: []string{mcVersion},
	}

	var result map[string]Version
	err := c.do(ctx, "POST", updateURL, reqBody, &result)
	return result, err
}

// GetVersionFromHashes identifies current mods
func (c *Client) CheckVersionsFromHashes(ctx context.Context, hashes []string) (map[string]Version, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	// Build the request
	reqBody := VersionRequest{
		Hashes:    hashes,
		Algorithm: "sha1",
	}

	var result map[string]Version
	err := c.do(ctx, "POST", versionsFilesURL, reqBody, &result)

	return result, err

}

func (c *Client) GetProjects(projectIDs []string) (map[string]ModrinthProject, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}

	idBytes, _ := json.Marshal(projectIDs)
	query := url.QueryEscape(string(idBytes))
	reqUrl := fmt.Sprintf(projectsURL, query)

	var result []ModrinthProject
	err := c.do(context.Background(), "GET", reqUrl, nil, &result)

	if err != nil {
		return nil, err
	}

	// Convert to map for easy lookup
	projectMap := make(map[string]ModrinthProject)
	for _, project := range result {
		projectMap[project.ID] = project
	}

	return projectMap, nil
}
