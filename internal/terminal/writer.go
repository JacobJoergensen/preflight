package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

type OutputWriter struct {
	w *bufio.Writer
}

func NewOutputWriter() *OutputWriter {
	return NewOutputWriterFor(os.Stdout)
}

func NewOutputWriterFor(w io.Writer) *OutputWriter {
	return &OutputWriter{
		w: bufio.NewWriter(w),
	}
}

func (ow *OutputWriter) Println(text string) bool {
	_, err := fmt.Fprintln(ow.w, text)

	if err != nil {
		return false
	}

	return ow.Flush()
}

func (ow *OutputWriter) Printf(format string, args ...any) bool {
	_, err := fmt.Fprintf(ow.w, format, args...)

	if err != nil {
		return false
	}

	return ow.Flush()
}

func (ow *OutputWriter) PrintNewLines(count int) bool {
	for range count {
		if _, err := fmt.Fprintln(ow.w, ""); err != nil {
			return false
		}
	}

	return ow.Flush()
}

func (ow *OutputWriter) Write(p []byte) (n int, err error) {
	return ow.w.Write(p)
}

func (ow *OutputWriter) Flush() bool {
	return ow.w.Flush() == nil
}
