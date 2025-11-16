package internal

import (
	"fmt"
	"io"
	"os"
)

// Writer provides methods for output operations that library code needs.
// This allows callers to control where and how output is written, rather than
// forcing library code to use global state like fmt.Print or log.Fatal.
type Writer interface {
	// Print writes a message to the output stream.
	Print(v ...interface{})

	// Printf writes a formatted message to the output stream.
	Printf(format string, v ...interface{})

	// Println writes a message with a newline to the output stream.
	Println(v ...interface{})

	// Warning writes a warning message to the output stream.
	Warning(v ...interface{})

	// Warningf writes a formatted warning message to the output stream.
	Warningf(format string, v ...interface{})

	// Fatal writes an error message and signals a fatal error.
	// Implementation should handle cleanup and termination appropriately.
	Fatal(v ...interface{})

	// Fatalf writes a formatted error message and signals a fatal error.
	// Implementation should handle cleanup and termination appropriately.
	Fatalf(format string, v ...interface{})

	// GetWriter returns the underlying io.Writer for direct writing.
	GetWriter() io.Writer
}

// StandardWriter implements Writer using standard output/error streams.
type StandardWriter struct {
	out io.Writer
	err io.Writer
}

// NewStandardWriter creates a Writer that outputs to stdout and stderr.
func NewStandardWriter() *StandardWriter {
	return &StandardWriter{
		out: os.Stdout,
		err: os.Stderr,
	}
}

// NewCustomWriter creates a Writer with custom output streams.
// The out stream is used for normal output, while err is used for warnings and fatal errors.
func NewCustomWriter(out, err io.Writer) *StandardWriter {
	return &StandardWriter{
		out: out,
		err: err,
	}
}

// Print writes a message to the output stream without adding a newline.
func (w *StandardWriter) Print(v ...interface{}) {
	fmt.Fprint(w.out, v...)
}

// Printf writes a formatted message to the output stream.
func (w *StandardWriter) Printf(format string, v ...interface{}) {
	fmt.Fprintf(w.out, format, v...)
}

// Println writes a message with a newline to the output stream.
func (w *StandardWriter) Println(v ...interface{}) {
	fmt.Fprintln(w.out, v...)
}

// Warning writes a warning message to the error stream with a "Warning: " prefix.
func (w *StandardWriter) Warning(v ...interface{}) {
	fmt.Fprint(w.err, "Warning: ")
	fmt.Fprintln(w.err, v...)
}

// Warningf writes a formatted warning message to the error stream with a "Warning: " prefix.
func (w *StandardWriter) Warningf(format string, v ...interface{}) {
	fmt.Fprintf(w.err, "Warning: "+format+"\n", v...)
}

// Fatal writes an error message to the error stream and exits the program with status 1.
func (w *StandardWriter) Fatal(v ...interface{}) {
	fmt.Fprintln(w.err, v...)
	os.Exit(1)
}

// Fatalf writes a formatted error message to the error stream and exits the program with status 1.
func (w *StandardWriter) Fatalf(format string, v ...interface{}) {
	fmt.Fprintf(w.err, format+"\n", v...)
	os.Exit(1)
}

// GetWriter returns the underlying io.Writer for direct writing to the output stream.
func (w *StandardWriter) GetWriter() io.Writer {
	return w.out
}
