
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "SKP_Silk_SDK_API.h"

#define MAX_INPUT_FRAMES        5

typedef struct silk_handle{
    SKP_SILK_SDK_DecControlStruct dec_ctrl;
    SKP_uint8 *dec_state;
} * silk_handle_t;

struct silk_handle *silk_decoder_init(void)
{
    silk_handle_t handle = calloc(1, sizeof(struct silk_handle));
    if (NULL == handle)
        return NULL;
    
    handle->dec_ctrl.API_sampleRate = 24000;
    handle->dec_ctrl.framesPerPacket = 1;

    SKP_int32 dec_size;
    SKP_Silk_SDK_Get_Decoder_Size(&dec_size);

    handle->dec_state = calloc(1, dec_size);
    if (NULL == handle->dec_state) {
        free(handle);
        return NULL;
    }

    SKP_Silk_SDK_InitDecoder(handle->dec_state);

    return handle;
}

void silk_decoder_deinit(struct silk_handle *h) {
    silk_handle_t handle = (silk_handle_t)h;

    if (handle) {
        if (handle->dec_state) {
            free(handle->dec_state);
            handle->dec_state = NULL;
        }
        free(handle);
    }
}

int silk_decoder_process(struct silk_handle *h, unsigned char *frame, int frame_size, unsigned char *output_payload, int output_len)
{
    silk_handle_t handle = (silk_handle_t)h;
    SKP_int16 *out_ptr = (SKP_int16 *)output_payload;
    SKP_int16 len = 0, total_len = 0;
    SKP_int32 frame_count = 0;
    do {
        SKP_Silk_SDK_Decode(handle->dec_state, &handle->dec_ctrl, 0, frame, frame_size, out_ptr, &len);
        frame_count++;
        out_ptr += len;
        total_len += len;
        if (frame_count > MAX_INPUT_FRAMES) {
            printf("frame_count > MAX_INPUT_FRAMES frame_count %d, total_len %d\n", frame_count, total_len);
            out_ptr = (SKP_int16 *)output_payload;
            total_len = 0;
            frame_count = 0;
        }
    } while (handle->dec_ctrl.moreInternalDecoderFrames);


    return total_len * 2;
}

int silk_decoder_set_sample_rate(struct silk_handle *h, int rate)
{
    silk_handle_t handle = (silk_handle_t)h;
    if (handle) {
        handle->dec_ctrl.API_sampleRate = rate;
    }

    return 0;
}
