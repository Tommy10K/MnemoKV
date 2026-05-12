package engine

import "errors"

// Sentinel errors used by command handlers to translate engine outcomes into
// RESP error replies in one place.
var (
	ErrWrongType    = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrNotInteger   = errors.New("ERR value is not an integer or out of range")
	ErrIntOverflow  = errors.New("ERR increment or decrement would overflow")
	ErrSyntax       = errors.New("ERR syntax error")
	ErrUnknownCmd   = errors.New("ERR unknown command")
	ErrWrongNumArgs = errors.New("ERR wrong number of arguments")
)
