package modrinth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckForUpdates(t *testing.T) {
	// 1. Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL and Method
		if r.URL.Path != "/version_files/update" {
			t.Errorf("Expected path /version_files/update, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		// Verify Request Body
		var req UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.GameVersions[0] != "1.21" || req.Loaders[0] != "fabric" {
			t.Errorf("Unexpected request filters: %v", req)
		}

		// Send mock response
		resp := map[string]Version{
			"hash123": {
				ID:            "versionABC",
				ProjectID:     "projectXYZ",
				VersionNumber: "2.0.0",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 2. Setup client pointing to mock
	client := NewClient("testuser", "testproj", "1.0")
	client.BaseUrl = server.URL

	// 3. Execute
	updates, err := client.CheckForUpdates(context.Background(), []string{"hash123"}, "1.21", "fabric")
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}

	// 4. Verify results
	if updates["hash123"].VersionNumber != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", updates["hash123"].VersionNumber)
	}
}

func TestCheckVersionsFromHashes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version_files" {
			t.Errorf("Expected path /version_files, got %s", r.URL.Path)
		}
		
		resp := map[string]Version{
			"hash123": {
				ID: "v1",
				ProjectID: "p1",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test", "test", "1.0")
	client.BaseUrl = server.URL

	versions, err := client.CheckVersionsFromHashes(context.Background(), []string{"hash123"})
	if err != nil {
		t.Fatalf("CheckVersionsFromHashes failed: %v", err)
	}

	if len(versions) != 1 || versions["hash123"].ID != "v1" {
		t.Errorf("Unexpected result: %+v", versions)
	}
}

func TestGetProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			t.Errorf("Expected path /projects, got %s", r.URL.Path)
		}
		
		// Modrinth returns an array for this endpoint
		resp := []ModrinthProject{
			{
				ID:    "p1",
				Title: "Test Mod",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test", "test", "1.0")
	client.BaseUrl = server.URL

	projects, err := client.GetProjects([]string{"p1"})
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}

	if projects["p1"].Title != "Test Mod" {
		t.Errorf("Expected Title 'Test Mod', got %s", projects["p1"].Title)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient("test", "test", "1.0")
	client.BaseUrl = server.URL

	_, err := client.GetProjects([]string{"p1"})
	if err == nil {
		t.Error("Expected error for 404 status, got nil")
	}
}
