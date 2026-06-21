package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	host := flag.String("host", "127.0.0.1", "API host")
	port := flag.Int("port", 7381, "API port")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: adminctl [-host H] [-port P] <command> [args]")
		fmt.Fprintln(os.Stderr, "commands: health, engine, cluster, metrics, snapshot, cluster-promote <slot>, cluster-assign-replica <slot> <node>, cluster-sync <slot> [node]")
		os.Exit(1)
	}

	base := fmt.Sprintf("http://%s:%d", *host, *port)
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := requestForArgs(base, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pretty map[string]interface{}
	if json.Unmarshal(body, &pretty) == nil {
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Println(string(body))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		os.Exit(1)
	}
}

func requestForCommand(base, command string) (*http.Request, error) {
	return requestForArgs(base, []string{command})
}

func requestForArgs(base string, args []string) (*http.Request, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	command := args[0]
	method := http.MethodGet
	var endpoint string
	var body io.Reader
	switch command {
	case "health":
		endpoint = "/health"
	case "engine":
		endpoint = "/engine/state"
	case "cluster":
		endpoint = "/cluster/state"
	case "metrics":
		endpoint = "/metrics/summary"
	case "snapshot":
		endpoint = "/admin/snapshot"
		method = http.MethodPost
	case "cluster-promote":
		if len(args) != 2 {
			return nil, fmt.Errorf("cluster-promote requires a slot")
		}
		slot, err := strconv.ParseUint(args[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid slot %q", args[1])
		}
		endpoint = "/cluster/promote"
		method = http.MethodPost
		body = strings.NewReader(fmt.Sprintf(`{"slot":%d}`, slot))
	case "cluster-assign-replica":
		if len(args) != 3 {
			return nil, fmt.Errorf("cluster-assign-replica requires a slot and node ID")
		}
		slot, err := strconv.ParseUint(args[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid slot %q", args[1])
		}
		payload, _ := json.Marshal(map[string]any{"slot": slot, "nodeId": args[2]})
		endpoint = "/cluster/replica"
		method = http.MethodPost
		body = bytes.NewReader(payload)
	case "cluster-sync":
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("cluster-sync requires a slot and optional node ID")
		}
		slot, err := strconv.ParseUint(args[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid slot %q", args[1])
		}
		payload := map[string]any{"slot": slot}
		if len(args) == 3 {
			payload["nodeId"] = args[2]
		}
		raw, _ := json.Marshal(payload)
		endpoint = "/cluster/sync"
		method = http.MethodPost
		body = bytes.NewReader(raw)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
	req, err := http.NewRequest(method, base+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
