package resp

import "errors"

// ErrProtocol indicates the input on the wire violated RESP2 framing rules.
// Callers (typically the connection loop) should treat this as fatal for the
// connection: there is no safe way to resync after a framing error.
var ErrProtocol = errors.New("resp: protocol error")

// ErrEmptyCommand is returned for an array of zero bulk strings, which is
// valid RESP but meaningless as a command.
var ErrEmptyCommand = errors.New("resp: empty command")

// ErrEmptyLine is returned for a blank inline command. The server can ignore
// it and continue because no framed request was started.
var ErrEmptyLine = errors.New("resp: empty inline command")
