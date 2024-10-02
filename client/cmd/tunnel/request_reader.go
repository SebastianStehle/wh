package tunnel

import (
	"fmt"
	"io"
)

type requestReader struct {
	curr   *requestChunk
	cursor int
	data   chan requestChunk
}

type requestChunk struct {
	data []byte
	eof  bool
}

func newRequestReader() *requestReader {
	return &requestReader{data: make(chan requestChunk, 1)}
}

func (r *requestReader) AppendData(data []byte, eof bool) {
	fmt.Printf("READ D\n")
	r.data <- requestChunk{data: data, eof: eof}
}

func (r *requestReader) Read(p []byte) (n int, err error) {
	if r.curr != nil {
		fmt.Printf("READ 1\n")
		return r.readNext(p)
	}

	// Just wait for the next chunk from the backend.
	for m := range r.data {
		r.curr = &m
		break
	}

	fmt.Printf("READ 2\n")
	return r.readNext(p)
}

func (r *requestReader) readNext(p []byte) (n int, err error) {
	data := r.curr.data
	n = 0
	if r.cursor < len(data) {
		slice := r.curr.data[r.cursor:]

		n = copy(p, slice)
		r.cursor += n
	}

	err = nil
	if r.cursor >= len(data) {
		if r.curr.eof {
			err = io.EOF
		}

		r.curr = nil
		r.cursor = 0
	}

	return n, err
}
