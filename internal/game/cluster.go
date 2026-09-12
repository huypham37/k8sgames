package game

import (
	"context"
	"io"
)

type Result struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
}

type TerminalSize struct {
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
}

type Terminal interface {
	io.ReadWriteCloser
	Resize(TerminalSize) error
}

type Cluster interface {
	Provision(context.Context, string, Challenge) error
	Inspect(context.Context, string, []string) Result
	OpenTerminal(context.Context, string, Challenge, TerminalSize) (Terminal, error)
	Delete(context.Context, string) error
	Cleanup(context.Context) error
	Ping(context.Context) error
}
