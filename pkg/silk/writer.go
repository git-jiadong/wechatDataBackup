package silk

import (
	"io"
)

type SilkWriter struct {
	output           io.Writer
	Decoder          *Decoder
	DecodedChunkSize int
}

func NewWriter(out io.Writer) *SilkWriter {
	writer := &SilkWriter{out, SilkInit(), 0}
	return writer
}

func (sw *SilkWriter) Write(p []byte) (int, error) {
	// fmt.Println("silk Write len:", len(p))
	out := sw.Decoder.Decode(p)
	sw.DecodedChunkSize = len(out)

	if sw.DecodedChunkSize > 0 {
		_, err := sw.output.Write(out)
		if err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (sw *SilkWriter) Close() error {
	out := sw.Decoder.Flush()
	if len(out) == 0 {
		return nil
	}
	sw.Decoder.Close()
	_, err := sw.output.Write(out)
	return err
}
