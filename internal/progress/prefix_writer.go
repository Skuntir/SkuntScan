package progress

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

type PrefixWriter struct {
	w      io.Writer
	prefix func() string

	mu  sync.Mutex
	buf bytes.Buffer
}

func NewPrefixWriter(w io.Writer, prefix func() string) *PrefixWriter {
	return &PrefixWriter{w: w, prefix: prefix}
}

func (p *PrefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n, err := p.buf.Write(b)
	for {
		line, ok := p.nextChunkLocked()
		if !ok {
			break
		}
		if line == "" {
			continue
		}
		if _, wErr := fmt.Fprintf(p.w, "%s%s\n", p.prefix(), line); wErr != nil && err == nil {
			err = wErr
		}
	}
	return n, err
}

func (p *PrefixWriter) Flush() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	rest := bytes.TrimRight(p.buf.Bytes(), "\r\n")
	if len(rest) == 0 {
		p.buf.Reset()
		return nil
	}
	p.buf.Reset()
	_, err := fmt.Fprintf(p.w, "%s%s\n", p.prefix(), string(rest))
	return err
}

func (p *PrefixWriter) nextChunkLocked() (string, bool) {
	data := p.buf.Bytes()
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			line := string(bytes.TrimRight(data[:i], "\r"))
			consume := 1
			if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				consume = 2
			}
			p.buf.Next(i + consume)
			return line, true
		}
	}
	return "", false
}

func TimePrefix(apex, tool, stream string) func() string {
	return func() string {
		return fmt.Sprintf("%s  %-8s  %-12s  %-6s  ", time.Now().Format("15:04:05"), apex, tool, stream)
	}
}
