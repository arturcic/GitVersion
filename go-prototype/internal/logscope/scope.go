// Package logscope formats the nested phases in the CLI transcript.
package logscope

import (
	"fmt"
	"time"
)

func Begin(write func(string), title string) (body func(string), end func()) {
	if write == nil {
		return nil, func() {}
	}
	started := time.Now()
	write("┌─── Begin: " + title)
	return func(message string) { write("│   " + message) }, func() {
		write(fmt.Sprintf("└─── End: %s (Took: %.2fms)", title, float64(time.Since(started))/float64(time.Millisecond)))
	}
}
