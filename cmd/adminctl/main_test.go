package main

import (
	"net/http"
	"testing"
)

func TestSnapshotCommandUsesAdminPostEndpoint(t *testing.T) {
	req, err := requestForCommand("http://127.0.0.1:7380", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.URL.Path != "/admin/snapshot" {
		t.Fatalf("request = %s %s", req.Method, req.URL.Path)
	}
}
