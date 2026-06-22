package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HealthResponse struct {
	Status string `json:"status"`
	NodeID string `json:"nodeId"`
	Mode   string `json:"mode"`
}

type ClusterStateResponse struct {
	Enabled         bool         `json:"enabled"`
	NodeID          string       `json:"nodeId"`
	ClusterID       string       `json:"clusterId"`
	SlotCount       uint32       `json:"slotCount"`
	MetadataVersion uint64       `json:"metadataVersion"`
	Slots           []SlotStatus `json:"slots"`
}

type SlotStatus struct {
	Number       uint32 `json:"number"`
	LeaderID     string `json:"leaderId"`
	ReplicaID    string `json:"replicaId"`
	Term         uint64 `json:"term"`
	ReplicaReady bool   `json:"replicaReady"`
}

type NodeAPI interface {
	Health(context.Context) (HealthResponse, error)
	ClusterState(context.Context) (ClusterStateResponse, error)
}

type NodeClient struct {
	baseURL string
	client  *http.Client
}

func NewNodeClient(address string, timeout time.Duration) (*NodeClient, error) {
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid node API address %q", address)
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	return &NodeClient{baseURL: strings.TrimRight(parsed.String(), "/"), client: &http.Client{Timeout: timeout}}, nil
}

func (c *NodeClient) Health(ctx context.Context) (HealthResponse, error) {
	var response HealthResponse
	err := c.getJSON(ctx, "/health", &response)
	return response, err
}

func (c *NodeClient) ClusterState(ctx context.Context) (ClusterStateResponse, error) {
	var response ClusterStateResponse
	err := c.getJSON(ctx, "/cluster/state", &response)
	return response, err
}

func (c *NodeClient) getJSON(ctx context.Context, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GET %s returned %s", path, response.Status)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 8<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode GET %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("decode GET %s: trailing JSON", path)
	}
	return nil
}
