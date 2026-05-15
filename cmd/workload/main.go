// Command workload is the synthetic traffic generator. It connects to a
// running MnemoKV node over RESP2 and drives one of the bundled profiles.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mnemokv/mnemokv/internal/workload"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:6380", "MnemoKV server address")
	profile := flag.String("profile", "mixed", "workload profile: strings|lists|zset|mixed")
	concurrency := flag.Int("concurrency", 8, "number of concurrent clients")
	duration := flag.Duration("duration", 10*time.Second, "run duration")
	keySpan := flag.Int("keyspan", 1000, "key cardinality")
	seed := flag.Int64("seed", time.Now().UnixNano(), "random seed")
	flag.Parse()

	prof, ok := workload.ProfileByName(*profile)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown profile %q\n", *profile)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("workload: profile=%s concurrency=%d duration=%s addr=%s", prof.Name, *concurrency, *duration, *addr)
	res, err := workload.Run(ctx, workload.RunOptions{
		Address:     *addr,
		Profile:     prof,
		Concurrency: *concurrency,
		Duration:    *duration,
		KeySpan:     *keySpan,
		Seed:        *seed,
	})
	if err != nil {
		log.Fatalf("workload: %v", err)
	}
	fmt.Println(res)
}
