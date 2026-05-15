package workload

import (
	"fmt"
	"math/rand"
	"strconv"
)

type RandSource interface {
	Key(prefix string) string
	CounterKey(prefix string) string
	Value(size int) string
	Score() string
}

type defaultRand struct {
	r       *rand.Rand
	keySpan int
}

func NewRand(seed int64, keySpan int) RandSource {
	if keySpan <= 0 {
		keySpan = 1000
	}
	return &defaultRand{r: rand.New(rand.NewSource(seed)), keySpan: keySpan}
}

func (d *defaultRand) Key(prefix string) string {
	return fmt.Sprintf("%s:%d", prefix, d.r.Intn(d.keySpan))
}

func (d *defaultRand) CounterKey(prefix string) string {
	return fmt.Sprintf("%s:%d", prefix, d.r.Intn(16))
}

func (d *defaultRand) Value(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[d.r.Intn(len(charset))]
	}
	return string(b)
}

func (d *defaultRand) Score() string {
	return strconv.FormatFloat(d.r.Float64()*1000, 'f', 2, 64)
}
