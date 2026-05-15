package workload

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type RunOptions struct {
	Address     string
	Profile     Profile
	Concurrency int
	Duration    time.Duration
	KeySpan     int
	Seed        int64
}

type Result struct {
	TotalOps   uint64
	Errors     uint64
	Duration   time.Duration
	Throughput float64
	AvgLatency time.Duration
}

func Run(ctx context.Context, opts RunOptions) (Result, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 8
	}
	if opts.Duration <= 0 {
		opts.Duration = 10 * time.Second
	}
	if opts.KeySpan <= 0 {
		opts.KeySpan = 1000
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Duration)
	defer cancel()

	var (
		ops      atomic.Uint64
		errs     atomic.Uint64
		latSumNs atomic.Int64
		wg       sync.WaitGroup
	)

	start := time.Now()
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			cli, err := Dial(opts.Address, 5*time.Second)
			if err != nil {
				errs.Add(1)
				return
			}
			defer cli.Close()

			rng := NewRand(opts.Seed+int64(workerID), opts.KeySpan)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				op := pickOperation(opts.Profile.Operations, rng)
				if op == nil {
					return
				}
				args := op.Build(rng)
				t0 := time.Now()
				if _, err := cli.Do(args...); err != nil {
					errs.Add(1)
					continue
				}
				latSumNs.Add(int64(time.Since(t0)))
				ops.Add(1)
			}
		}(i)
	}

	wg.Wait()
	dur := time.Since(start)
	totalOps := ops.Load()
	avg := time.Duration(0)
	if totalOps > 0 {
		avg = time.Duration(latSumNs.Load() / int64(totalOps))
	}
	return Result{
		TotalOps:   totalOps,
		Errors:     errs.Load(),
		Duration:   dur,
		Throughput: float64(totalOps) / dur.Seconds(),
		AvgLatency: avg,
	}, nil
}

func pickOperation(ops []Operation, r RandSource) *Operation {
	total := 0
	for i := range ops {
		total += ops[i].Weight
	}
	if total == 0 {
		return nil
	}
	pick := pickInt(r, total)
	cur := 0
	for i := range ops {
		cur += ops[i].Weight
		if pick < cur {
			return &ops[i]
		}
	}
	return &ops[len(ops)-1]
}

func pickInt(r RandSource, max int) int {
	s := r.Value(4)
	x := 0
	for _, c := range s {
		x = (x*131 + int(c)) & 0x7fffffff
	}
	return x % max
}

func (r Result) String() string {
	return fmt.Sprintf("ops=%d errors=%d duration=%s throughput=%.1f ops/s avg_latency=%s",
		r.TotalOps, r.Errors, r.Duration.Round(time.Millisecond),
		r.Throughput, r.AvgLatency.Round(time.Microsecond))
}
