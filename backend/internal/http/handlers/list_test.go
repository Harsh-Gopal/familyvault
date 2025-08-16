package handlers

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

func TestListAndDownloadFlow(t *testing.T) {
	// Setup drive and session
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)
	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	// Upload a file
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		part, _ := writer.CreateFormFile("file", "doc.txt")
		part.Write([]byte("hello secure world"))
		writer.WriteField("tags", "example")
		writer.Close()
	}()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/upload", pr)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// List files
	listResp, err := http.Get(srv.URL + "/list?session_id=" + s.ID)
	if err != nil {
		t.Fatalf("list get: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status: %d", listResp.StatusCode)
	}
	var items []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&items); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(items) != 1 || items[0]["filename"].(string) != "doc.txt" {
		t.Fatalf("unexpected list items: %+v", items)
	}

	// Download file and verify plaintext
	dreq, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename=doc.txt", nil)
	dreq.Header.Set("X-Session-ID", s.ID)
	dresp, err := http.DefaultClient.Do(dreq)
	if err != nil {
		t.Fatalf("download do: %v", err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != http.StatusOK {
		t.Fatalf("download status: %d", dresp.StatusCode)
	}
	body, _ := io.ReadAll(dresp.Body)
	if string(body) != "hello secure world" {
		t.Fatalf("download content mismatch: %q", string(body))
	}
}
