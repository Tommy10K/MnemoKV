package logging

import (
	"log"
	"strings"
	"sync/atomic"
)

type Level int32

const (
	Debug Level = iota
	Info
	Warn
	Error
	Off
)

var current atomic.Int32

func init() {
	current.Store(int32(Info))
}

func ParseLevel(raw string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return Debug, true
	case "", "info":
		return Info, true
	case "warn", "warning":
		return Warn, true
	case "error":
		return Error, true
	case "off", "none":
		return Off, true
	default:
		return Info, false
	}
}

func SetLevel(raw string) bool {
	level, ok := ParseLevel(raw)
	if !ok {
		return false
	}
	current.Store(int32(level))
	return true
}

func Debugf(format string, args ...any) { logf(Debug, format, args...) }
func Infof(format string, args ...any)  { logf(Info, format, args...) }
func Warnf(format string, args ...any)  { logf(Warn, format, args...) }
func Errorf(format string, args ...any) { logf(Error, format, args...) }

func logf(level Level, format string, args ...any) {
	configured := Level(current.Load())
	if configured == Off || level < configured {
		return
	}
	log.Printf(format, args...)
}
