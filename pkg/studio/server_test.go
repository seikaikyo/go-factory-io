package studio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_StaticFiles(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Test index.html
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET / status: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("missing Content-Type for /")
	}

	// Test CSS
	resp, err = http.Get(ts.URL + "/studio.css")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /studio.css status: %d", resp.StatusCode)
	}

	// Test JS
	resp, err = http.Get(ts.URL + "/studio.js")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /studio.js status: %d", resp.StatusCode)
	}
}

func TestServer_StatusAPI(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Error("expected success=true")
	}
}

func TestServer_ReportAPI(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/report")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/report: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Error("expected success=true")
	}
	data := result["data"].(map[string]interface{})
	if data["totalExpected"].(float64) == 0 {
		t.Error("expected non-zero totalExpected in report")
	}
}

func TestServer_TraceAPI(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/trace")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/trace: %d", resp.StatusCode)
	}
}
