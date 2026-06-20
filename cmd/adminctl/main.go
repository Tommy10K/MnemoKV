package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	host := flag.String("host", "127.0.0.1", "API host")
	port := flag.Int("port", 7381, "API port")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: adminctl [-host H] [-port P] <command>")
		fmt.Fprintln(os.Stderr, "commands: health, engine, cluster, metrics, snapshot")
		os.Exit(1)
	}

	base := fmt.Sprintf("http://%s:%d", *host, *port)
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := requestForCommand(base, args[0])
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
	method := http.MethodGet
	var endpoint string
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
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
	req, err := http.NewRequest(method, base+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return req, nil
}
