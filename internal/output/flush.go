package output

import "io"

type flusher interface {
	Flush() error
}

func flushIfPossible(w io.Writer) error {
	f, ok := w.(flusher)
	if !ok {
		return nil
	}
	return f.Flush()
}
