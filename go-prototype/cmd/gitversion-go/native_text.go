package main

import (
	"bytes"
	"io"
	"runtime"
)

func nativeText(data []byte) []byte {
	if runtime.GOOS != "windows" || !bytes.Contains(data, []byte{'\n'}) {
		return data
	}
	return bytes.ReplaceAll(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("\n"), []byte("\r\n"))
}

// rawTextWriter preserves serialization that already defines its own newlines.
func rawTextWriter(w io.Writer) io.Writer {
	if native, ok := w.(nativeTextWriter); ok {
		return native.Writer
	}
	return w
}

type nativeTextWriter struct {
	io.Writer
}

func (w nativeTextWriter) Write(data []byte) (int, error) {
	rendered := nativeText(data)
	n, err := w.Writer.Write(rendered)
	if err != nil || n != len(rendered) {
		if err == nil {
			err = io.ErrShortWrite
		}
		return 0, err
	}
	return len(data), nil
}
