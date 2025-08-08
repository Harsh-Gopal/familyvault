package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

func TestDeleteFlow(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)
	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	srv := newTestServer()
	defer srv.Close()

	// Upload a file
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "to-delete.txt")
	fw.Write([]byte("payload"))
	mw.Close()
	ureq, _ := http.NewRequest(http.MethodPost, srv.URL+"/upload", &body)
	ureq.Header.Set("Content-Type", mw.FormDataContentType())
	ureq.Header.Set("X-Session-ID", s.ID)
	uresp, err := http.DefaultClient.Do(ureq)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if uresp.StatusCode != http.StatusOK {
		t.Fatalf("upload status %d", uresp.StatusCode)
	}
	uresp.Body.Close()

	// Delete via /delete
	delReq := map[string]string{"filename": "to-delete.txt"}
	buf, _ := json.Marshal(delReq)
	dreq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/delete?session_id="+s.ID, bytes.NewReader(buf))
	dreq.Header.Set("Content-Type", "application/json")
	dresp, err := http.DefaultClient.Do(dreq)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if dresp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(dresp.Body)
		t.Fatalf("delete status %d: %s", dresp.StatusCode, string(b))
	}
	dresp.Body.Close()

	// List should be empty
	lresp, err := http.Get(srv.URL + "/list?session_id=" + s.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer lresp.Body.Close()
	var items []map[string]any
	if err := json.NewDecoder(lresp.Body).Decode(&items); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d", len(items))
	}

	// Download should 404
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename=to-delete.txt", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after deletion, got %d", resp.StatusCode)
	}
}
