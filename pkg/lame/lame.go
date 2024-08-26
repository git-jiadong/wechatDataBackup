package lame

// http://www.leidinger.net/lame/doxy/html/lame_8h-source.html

/*
#cgo CFLAGS: -DHAVE_CONFIG_H -I./clame
#cgo LDFLAGS: -lm
#include "lame.h"
*/
import "C"

import (
	"runtime"
	"unsafe"
)

type Handle *C.struct_lame_global_struct

const (
	STEREO        = C.STEREO
	JOINT_STEREO  = C.JOINT_STEREO
	DUAL_CHANNEL  = C.DUAL_CHANNEL /* LAME doesn't supports this! */
	MONO          = C.MONO
	NOT_SET       = C.NOT_SET
	MAX_INDICATOR = C.MAX_INDICATOR
	BIT_DEPTH     = 16

	VBR_OFF     = C.vbr_off
	VBR_RH      = C.vbr_rh
	VBR_ABR     = C.vbr_abr
	VBR_MTRH    = C.vbr_mtrh
	VBR_DEFAULT = C.vbr_default

	MAX_FRAME_SIZE = 2880
)

const (
	ABR_8   = C.ABR_8
	ABR_320 = C.ABR_320
	V9      = C.V9
	VBR_10  = C.VBR_10
	V8      = C.V8
	VBR_20  = C.VBR_20
	V7      = C.V7
	VBR_30  = C.VBR_30
	V6      = C.V6
	VBR_40  = C.VBR_40
	V5      = C.V5
	VBR_50  = C.VBR_50
	V4      = C.V4
	VBR_60  = C.VBR_60
	V3      = C.V3
	VBR_70  = C.VBR_70
	V2      = C.V2
	VBR_80  = C.VBR_80
	V1      = C.V1
	VBR_90  = C.VBR_90
	V0      = C.V0
	VBR_100 = C.VBR_100
)

type Encoder struct {
	handle    Handle
	remainder []byte
	closed    bool
}

func Init() *Encoder {
	handle := C.lame_init()
	encoder := &Encoder{handle, make([]byte, 0), false}
	runtime.SetFinalizer(encoder, finalize)
	return encoder
}

func (e *Encoder) SetVBR(mode C.vbr_mode) {
	C.lame_set_VBR(e.handle, mode)
}

func (e *Encoder) SetVBRAverageBitRate(averageBitRate int) {
	C.lame_set_VBR_mean_bitrate_kbps(e.handle, C.int(averageBitRate))
}

func (e *Encoder) SetVBRQuality(quality int) {
	C.lame_set_VBR_q(e.handle, C.int(quality))
}

func (e *Encoder) SetLowPassFrequency(frequency int) {
	// Frequency in Hz
	C.lame_set_lowpassfreq(e.handle, C.int(frequency))
}

func (e *Encoder) SetNumChannels(num int) {
	C.lame_set_num_channels(e.handle, C.int(num))
}

func (e *Encoder) SetInSamplerate(sampleRate int) {
	C.lame_set_in_samplerate(e.handle, C.int(sampleRate))
}

func (e *Encoder) SetBitrate(bitRate int) {
	C.lame_set_brate(e.handle, C.int(bitRate))
}

func (e *Encoder) SetMode(mode C.MPEG_mode) {
	C.lame_set_mode(e.handle, mode)
}

func (e *Encoder) SetQuality(quality int) {
	C.lame_set_quality(e.handle, C.int(quality))
}

func (e *Encoder) InitId3Tag() {
	C.id3tag_init(e.handle)
}

func (e *Encoder) SetWriteId3tagAutomatic(automaticWriteTag int) {
	C.lame_set_write_id3tag_automatic(e.handle, C.int(automaticWriteTag))
}

func (e *Encoder) ID3TagAddV2() {
	C.id3tag_add_v2(e.handle)
}

func (e *Encoder) SetbWriteVbrTag(writeVbrTag int) {
	C.lame_set_bWriteVbrTag(e.handle, C.int(writeVbrTag))
}

func (e *Encoder) GetLametagFrame() []byte {
	tagFrame := make([]byte, MAX_FRAME_SIZE)
	tagFrameLen := C.lame_get_lametag_frame(e.handle, (*C.uchar)(unsafe.Pointer(&tagFrame[0])), C.size_t(len(tagFrame)))

	return tagFrame[0:tagFrameLen]
}

func (e *Encoder) InitParams() int {
	retcode := C.lame_init_params(e.handle)
	return int(retcode)
}

func (e *Encoder) NumChannels() int {
	n := C.lame_get_num_channels(e.handle)
	return int(n)
}

func (e *Encoder) Bitrate() int {
	br := C.lame_get_brate(e.handle)
	return int(br)
}

func (e *Encoder) Mode() int {
	m := C.lame_get_mode(e.handle)
	return int(m)
}

func (e *Encoder) Quality() int {
	q := C.lame_get_quality(e.handle)
	return int(q)
}

func (e *Encoder) InSamplerate() int {
	sr := C.lame_get_in_samplerate(e.handle)
	return int(sr)
}

func (e *Encoder) Encode(buf []byte) []byte {

	if len(e.remainder) > 0 {
		buf = append(e.remainder, buf...)
	}

	if len(buf) == 0 {
		return make([]byte, 0)
	}

	blockAlign := BIT_DEPTH / 8 * e.NumChannels()

	remainBytes := len(buf) % blockAlign
	if remainBytes > 0 {
		e.remainder = buf[len(buf)-remainBytes : len(buf)]
		buf = buf[0 : len(buf)-remainBytes]
	} else {
		e.remainder = make([]byte, 0)
	}

	numSamples := len(buf) / blockAlign
	estimatedSize := int(1.25*float64(numSamples) + 7200)
	out := make([]byte, estimatedSize)

	cBuf := (*C.short)(unsafe.Pointer(&buf[0]))
	cOut := (*C.uchar)(unsafe.Pointer(&out[0]))

	var bytesOut C.int

	if e.NumChannels() == 1 {
		bytesOut = C.int(C.lame_encode_buffer(
			e.handle,
			cBuf,
			nil,
			C.int(numSamples),
			cOut,
			C.int(estimatedSize),
		))
	} else {
		bytesOut = C.int(C.lame_encode_buffer_interleaved(
			e.handle,
			cBuf,
			C.int(numSamples),
			cOut,
			C.int(estimatedSize),
		))
	}
	return out[0:bytesOut]

}

func (e *Encoder) Flush() []byte {
	estimatedSize := 7200
	out := make([]byte, estimatedSize)
	cOut := (*C.uchar)(unsafe.Pointer(&out[0]))
	bytesOut := C.int(C.lame_encode_flush(
		e.handle,
		cOut,
		C.int(estimatedSize),
	))

	return out[0:bytesOut]
}

func (e *Encoder) Close() {
	if e.closed {
		return
	}
	C.lame_close(e.handle)
	e.closed = true
}

func finalize(e *Encoder) {
	e.Close()
}
