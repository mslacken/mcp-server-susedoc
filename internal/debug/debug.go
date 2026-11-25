// Package debug provides the diagnostic output shared by all packages. It
// imports nothing from this module, so every package can use it without
// creating import cycles.
package debug

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

var (
	enabled         atomic.Bool
	silenceWarnings atomic.Bool
	mu              sync.Mutex
	out             io.Writer = os.Stderr
)

// SetEnabled switches diagnostic output on or off.
func SetEnabled(on bool) {
	enabled.Store(on)
}

// Enabled reports whether diagnostic output is switched on.
func Enabled() bool {
	return enabled.Load()
}

// SetOutput redirects diagnostic output, stderr by default.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
}

// SetSilence disables output for the warnings
func SetSilence(silence bool) {
	silenceWarnings.Store(silence)
}

// Printf writes one line tagged with component if output is enabled.
func Printf(component, format string, args ...any) {
	if !enabled.Load() {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_, _ = fmt.Fprintf(out, "[debug] "+component+": "+format+"\n", args...)
}

// New returns a printf style function bound to component, meant to be kept
// in a package level variable:
//
//	var debugf = debug.New("loader")
func New(component string) func(format string, args ...any) {
	return func(format string, args ...any) {
		Printf(component, format, args...)
	}
}

// warnings counts the lines written by Warnf, for Warnings.
var warnings atomic.Int64

// Warnf writes one line tagged with component, whether output is enabled or
// not, for content that is dropped while reading the documents.
func Warnf(component, format string, args ...any) {
	warnings.Add(1)
	if !silenceWarnings.Load() {
		mu.Lock()
		defer mu.Unlock()
		_, _ = fmt.Fprintf(out, "[warn] "+component+": "+format+"\n", args...)
	}
}

// Warn writes err as a warning, as Warnf does, one line for each of the
// errors joined in err.
func Warn(component string, err error) {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range j.Unwrap() {
			Warn(component, e)
		}
		return
	}
	if err != nil {
		Warnf(component, "%v", err)
	}
}

// NewWarn returns a printf style function bound to component that writes
// warnings, as New does for diagnostic output.
func NewWarn(component string) func(format string, args ...any) {
	return func(format string, args ...any) {
		Warnf(component, format, args...)
	}
}

// Warnings returns the number of warnings written so far.
func Warnings() int {
	return int(warnings.Load())
}
