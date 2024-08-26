package lame

import (
	"io"
)

type LameWriter struct {
	output           io.Writer
	Encoder          *Encoder
	EncodedChunkSize int
}

func NewWriter(out io.Writer) *LameWriter {
	writer := &LameWriter{out, Init(), 0}
	return writer
}

func (lw *LameWriter) Write(p []byte) (int, error) {
	// fmt.Println("lame Write len:", len(p))
	out := lw.Encoder.Encode(p)
	lw.EncodedChunkSize = len(out)

	if lw.EncodedChunkSize > 0 {
		_, err := lw.output.Write(out)
		if err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (lw *LameWriter) Close() error {
	out := lw.Encoder.Flush()
	if len(out) == 0 {
		return nil
	}
	lw.Encoder.Close()
	_, err := lw.output.Write(out)
	return err
}
