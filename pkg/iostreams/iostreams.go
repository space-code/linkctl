package iostreams

import (
	"bytes"
	"io"
	"os"

	"github.com/fatih/color"
)

type fileWriter interface {
	io.Writer
	Fd() uintptr
}

type fileReader interface {
	io.ReadCloser
	Fd() uintptr
}

type IOStreams struct {
	In     fileReader
	Out    fileWriter
	ErrOut fileWriter

	colorEnabled bool
}

func System() *IOStreams {
	ios := &IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	ios.colorEnabled = !color.NoColor

	return ios
}

func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	io := &IOStreams{
		In: &fdReader{
			fd:         0,
			ReadCloser: io.NopCloser(in),
		},
		Out:          &fdWriter{fd: 1, Writer: out},
		ErrOut:       &fdWriter{fd: 2, Writer: errOut},
		colorEnabled: false,
	}

	return io, in, out, errOut
}

func (ios *IOStreams) ColorEnabled() bool {
	return ios.colorEnabled
}

type ColorScheme struct {
	enabled bool
}

func (ios *IOStreams) ColorScheme() *ColorScheme {
	return &ColorScheme{enabled: ios.colorEnabled}
}

func (cs *ColorScheme) format(c *color.Color, s string) string {
	if !cs.enabled {
		return s
	}
	return c.Sprint(s)
}

func (cs *ColorScheme) Bold(s string) string {
	return cs.format(color.New(color.Bold), s)
}

func (cs *ColorScheme) Green(s string) string {
	return cs.format(color.New(color.FgGreen), s)
}

func (cs *ColorScheme) Cyan(s string) string {
	return cs.format(color.New(color.FgCyan), s)
}

func (cs *ColorScheme) Yellow(s string) string {
	return cs.format(color.New(color.FgYellow), s)
}

func (cs *ColorScheme) Red(s string) string {
	return cs.format(color.New(color.FgRed), s)
}

func (cs *ColorScheme) Muted(s string) string {
	return cs.format(color.New(color.FgHiBlack), s)
}

type fdWriter struct {
	io.Writer
	fd uintptr
}

func (w *fdWriter) Fd() uintptr {
	return w.fd
}

type fdReader struct {
	io.ReadCloser
	fd uintptr
}

func (r *fdReader) Fd() uintptr {
	return r.fd
}
