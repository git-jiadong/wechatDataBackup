#ifndef __SKP_SILK_SDK_H
#define __SKP_SILK_SDK_H

struct silk_handle;

struct silk_handle *silk_decoder_init(void);
void silk_decoder_deinit(struct silk_handle *h);
int silk_decoder_process(struct silk_handle *h, unsigned char *frame, int frame_size, unsigned char *output_payload, int output_len);
int silk_decoder_set_sample_rate(struct silk_handle *h, int rate);

#endif /* __SKP_SILK_SDK_H */
