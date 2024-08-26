package silk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"
)

/*
#cgo CFLAGS: -I./csilk/
#include <stdlib.h>
#include "skp_silk_sdk.h"
*/
import "C"

const (
	MAX_BYTES_PER_FRAME = 1024
	MAX_INPUT_FRAMES    = 5
	MAX_FRAME_LENGTH    = 480
	FRAME_LENGTH_MS     = 20
	MAX_API_FS_KHZ      = 48
)

type Handle *C.struct_silk_handle

type Decoder struct {
	handle    Handle
	remainder []byte
	closed    bool
	foundHead bool
}

func SilkInit() *Decoder {
	h := C.silk_decoder_init()
	decoder := &Decoder{h, make([]byte, 0), false, false}

	return decoder
}

func (d *Decoder) SetSampleRate(rate int) {
	C.silk_decoder_set_sample_rate(d.handle, C.int(rate))
}

func (d *Decoder) Decode(buf []byte) []byte {
	d.remainder = append(d.remainder, buf...)

	if len(buf) == 0 || d.closed {
		return make([]byte, 0)
	}

	if !d.foundHead {
		var head []byte
		if d.remainder[0] == 0x02 {
			head = d.remainder[1:10]
			d.remainder = d.remainder[10:]
		} else {
			head = d.remainder[0:9]
			d.remainder = d.remainder[9:]
		}

		if string(head) == "#!SILK_V3" {
			d.foundHead = true
		} else {
			fmt.Println("not found head")
			return make([]byte, 0)
		}
	}

	// fmt.Println("remainder:", d.remainder[0:10])
	out := make([]byte, 0)
	for {
		if len(d.remainder) < 2 {
			break
		}
		buffer := bytes.NewBuffer(d.remainder[0:2])
		// fmt.Println("remainder:", d.remainder[0:2])

		var nlen int16
		if err := binary.Read(buffer, binary.LittleEndian, &nlen); err != nil {
			fmt.Println("Error reading int16:", err)
			d.remainder = d.remainder[2:]
			return make([]byte, 0)
		}

		if nlen <= 0 {
			fmt.Println("d.remainder:", d.remainder)
			d.remainder = d.remainder[2:]
			continue
		}
		// fmt.Println("nlen:", nlen)

		if len(d.remainder) < (int(nlen) + 2) {
			break
		}

		frame := d.remainder[2 : 2+nlen]
		// fmt.Println("frame:", frame)
		payload_len := ((FRAME_LENGTH_MS * MAX_API_FS_KHZ) << 1) * MAX_INPUT_FRAMES
		payload := make([]byte, payload_len)
		cframe := (*C.uchar)(unsafe.Pointer(&frame[0]))
		cpayload := (*C.uchar)(unsafe.Pointer(&payload[0]))
		outlen := C.int(C.silk_decoder_process(d.handle, cframe, C.int(nlen), cpayload, C.int(payload_len)))
		append_payload := payload[0:outlen]
		out = append(out, append_payload...)
		d.remainder = d.remainder[2+nlen:]
	}

	return out
}

func (d *Decoder) Close() {
	if !d.closed {
		d.closed = true
		C.silk_decoder_deinit(d.handle)
	}
}

func (d *Decoder) Flush() []byte {
	var tmp []byte
	return d.Decode(tmp)
}
