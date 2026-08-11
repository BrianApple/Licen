/* sha256.h - SHA-256 与 HMAC-SHA256（零依赖实现，标准 FIPS 180-4） */
#ifndef LICEN_SHA256_H
#define LICEN_SHA256_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct licen_sha256_ctx {
    uint32_t state[8];
    uint64_t bitlen;
    uint8_t data[64];
    size_t datalen;
} licen_sha256_ctx;

void licen_sha256_init(licen_sha256_ctx *ctx);
void licen_sha256_update(licen_sha256_ctx *ctx, const void *data, size_t len);
void licen_sha256_final(licen_sha256_ctx *ctx, uint8_t out[32]);

/* 计算 HMAC-SHA256，输出 32 字节 */
void licen_hmac_sha256(const uint8_t *key, size_t key_len,
                       const uint8_t *msg, size_t msg_len,
                       uint8_t out[32]);

/* HMAC 十六进制字符串（小写），out 需 >= 65 字节 */
void licen_hmac_sha256_hex(const char *key, const char *msg, char *out);

#ifdef __cplusplus
}
#endif

#endif /* LICEN_SHA256_H */
