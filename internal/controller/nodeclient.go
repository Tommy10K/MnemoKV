package controller

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	baseURL       string
	client        *http.Client
	signingSecret []byte
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

func NewAuthenticatedNodeClient(address string, timeout time.Duration, secret string) (*NodeClient, error) {
	client, err := NewNodeClient(address, timeout)
	if err != nil {
		return nil, err
	}
	client.signingSecret = []byte(secret)
	return client, nil
}

const (
	ControlIndexHeader     = "X-MnemoKV-Control-Index"
	ControlSignatureHeader = "X-MnemoKV-Control-Signature"
)

type ClusterAdminResponse struct {
	MetadataVersion uint64     `json:"metadataVersion"`
	Slot            SlotStatus `json:"slot"`
	FailedPeers     []string   `json:"failedPeers,omitempty"`
}

type AdminNodeAPI interface {
	NodeAPI
	Promote(context.Context, uint32, uint64) (ClusterAdminResponse, error)
	AssignReplica(context.Context, uint32, string, uint64) (ClusterAdminResponse, error)
	SyncReplica(context.Context, uint32, string, uint64) (ClusterAdminResponse, error)
}

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string { return e.Status }

func (c *NodeClient) Promote(ctx context.Context, slot uint32, controlIndex uint64) (ClusterAdminResponse, error) {
	return c.postAdmin(ctx, "/cluster/promote", struct {
		Slot uint32 `json:"slot"`
	}{slot}, controlIndex)
}

func (c *NodeClient) AssignReplica(ctx context.Context, slot uint32, nodeID string, controlIndex uint64) (ClusterAdminResponse, error) {
	return c.postAdmin(ctx, "/cluster/replica", struct {
		Slot   uint32 `json:"slot"`
		NodeID string `json:"nodeId"`
	}{slot, nodeID}, controlIndex)
}

func (c *NodeClient) SyncReplica(ctx context.Context, slot uint32, nodeID string, controlIndex uint64) (ClusterAdminResponse, error) {
	return c.postAdmin(ctx, "/cluster/sync", struct {
		Slot   uint32 `json:"slot"`
		NodeID string `json:"nodeId"`
	}{slot, nodeID}, controlIndex)
}

func (c *NodeClient) postAdmin(ctx context.Context, path string, payload any, controlIndex uint64) (ClusterAdminResponse, error) {
	var result ClusterAdminResponse
	body, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	index := strconv.FormatUint(controlIndex, 10)
	request.Header.Set(ControlIndexHeader, index)
	request.Header.Set(ControlSignatureHeader, signControlRequest(c.signingSecret, http.MethodPost, path, body, index))
	response, err := c.client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return result, &HTTPStatusError{StatusCode: response.StatusCode, Status: response.Status}
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&result); err != nil {
		return result, err
	}
	return result, nil
}

func signControlRequest(secret []byte, method, path string, body []byte, index string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(method))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(path))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(index))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
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
