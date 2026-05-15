package engine

var writeCommands = map[string]struct{}{
	"SET":      {},
	"DEL":      {},
	"EXPIRE":   {},
	"INCR":     {},
	"LPUSH":    {},
	"RPUSH":    {},
	"LPOP":     {},
	"RPOP":     {},
	"ZADD":     {},
	"FLUSHDB":  {},
	"FLUSHALL": {},
}

func IsWriteCommand(name string) bool {
	_, ok := writeCommands[name]
	return ok
}
