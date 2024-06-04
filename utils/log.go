package utils

import (
	"io"
	"log"
	"os"
)

var (
	SLogger = NewSessionLog(os.Stderr)
)

// Log implement the log.Logger
type Log struct {
	*log.Logger
}

// NewSessionLog set io.Writer to create a Logger for session.
func NewSessionLog(out io.Writer) *Log {
	sl := new(Log)
	sl.Logger = log.New(out, "[SESSION]", 1e9)
	return sl
}
