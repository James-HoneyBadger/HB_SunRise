// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

// Package log is a slim stand-in for honey-ledger's pkg/log. It offers the
// context-aware log functions HB SunRise binds to the money-engine extraction,
// without the logrus and settings machinery the server needs. SetQuiet
// discards all output, so command line tools that report their own results are
// not cluttered by the log lines of the libraries they call.
package log

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/core"
)

var (
	mu     sync.Mutex
	output io.Writer = os.Stdout
)

// SetQuiet discards all log output.
func SetQuiet() {
	mu.Lock()
	defer mu.Unlock()

	output = io.Discard
}

func writef(level string, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	fmt.Fprintf(output, "%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, fmt.Sprintf(format, args...))
}

// Infof writes an information line
func Infof(ctx core.Context, format string, args ...any) {
	writef("INFO", format, args...)
}

// Warnf writes a warning line
func Warnf(ctx core.Context, format string, args ...any) {
	writef("WARN", format, args...)
}

// Errorf writes an error line
func Errorf(ctx core.Context, format string, args ...any) {
	writef("ERROR", format, args...)
}
