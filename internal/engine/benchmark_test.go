package engine

import (
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func BenchmarkSET(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	cmd := &resp.Command{Name: "SET", Args: [][]byte{[]byte("bench-key"), []byte("bench-value")}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Execute(cmd)
	}
}

func BenchmarkGET(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	e.Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte("bench-key"), []byte("bench-value")}})
	cmd := &resp.Command{Name: "GET", Args: [][]byte{[]byte("bench-key")}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Execute(cmd)
	}
}

func BenchmarkINCR(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	cmd := &resp.Command{Name: "INCR", Args: [][]byte{[]byte("counter")}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Execute(cmd)
	}
}

func BenchmarkLPUSH(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	cmd := &resp.Command{Name: "LPUSH", Args: [][]byte{[]byte("list"), []byte("item")}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Execute(cmd)
	}
}

func BenchmarkZADD(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	cmd := &resp.Command{Name: "ZADD", Args: [][]byte{[]byte("zset"), []byte("1.5"), []byte("member")}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Execute(cmd)
	}
}

func BenchmarkSETParallel(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	b.RunParallel(func(pb *testing.PB) {
		cmd := &resp.Command{Name: "SET", Args: [][]byte{[]byte("pkey"), []byte("pval")}}
		for pb.Next() {
			e.Execute(cmd)
		}
	})
}

func BenchmarkGETParallel(b *testing.B) {
	e := New(config.EngineConfig{StripeCount: 32, EvictionPolicy: "noeviction"})
	e.Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte("pkey"), []byte("pval")}})
	b.RunParallel(func(pb *testing.PB) {
		cmd := &resp.Command{Name: "GET", Args: [][]byte{[]byte("pkey")}}
		for pb.Next() {
			e.Execute(cmd)
		}
	})
}
