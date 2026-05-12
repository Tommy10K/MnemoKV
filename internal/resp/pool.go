package resp

// Pool exposes the package-level command pool through a value type so callers
// (the server, tests, future replication code) can plug their own pool in
// later if profiling justifies it. The default implementation just delegates
// to acquireCommand / Release.
type Pool struct{}

// Acquire returns a fresh Command ready to be filled in by the parser.
func (Pool) Acquire() *Command { return acquireCommand() }

// Release returns the command to the pool.
func (Pool) Release(c *Command) { Release(c) }
