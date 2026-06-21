package main

import (
	"encoding/json"
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

func TestClusterAdminCommands(t *testing.T) {
	tests := []struct {
		args []string
		path string
		node string
	}{
		{args: []string{"cluster-promote", "7"}, path: "/cluster/promote"},
		{args: []string{"cluster-assign-replica", "8", "node-3"}, path: "/cluster/replica", node: "node-3"},
		{args: []string{"cluster-sync", "9", "node-2"}, path: "/cluster/sync", node: "node-2"},
	}
	for _, tc := range tests {
		req, err := requestForArgs("http://127.0.0.1:7380", tc.args)
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != http.MethodPost || req.URL.Path != tc.path {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		var body struct {
			Slot   uint32 `json:"slot"`
			NodeID string `json:"nodeId"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.NodeID != tc.node {
			t.Fatalf("nodeId = %q, want %q", body.NodeID, tc.node)
		}
	}
}
