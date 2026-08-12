/* http.h - HTTP 传输抽象（libcurl 或纯 POSIX socket 两种实现） */
#ifndef LICEN_HTTP_H
#define LICEN_HTTP_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct licen_http licen_http_t;

/* 创建 HTTP 客户端（含超时配置）。失败返回 NULL。 */
licen_http_t *licen_http_new(long connect_timeout_ms);

void licen_http_free(licen_http_t *h);

/*
 * POST/GET JSON 请求。
 * 成功返回 0，*out 指向 malloc 的响应体（调用方用 licen_http_free_resp 释放）；
 * 失败返回 -1。
 */
int licen_http_post(licen_http_t *h, const char *url, const char *path,
                    const char *json_body, char **out);
int licen_http_get(licen_http_t *h, const char *url, const char *path, char **out);

void licen_http_free_resp(char *resp);

#ifdef __cplusplus
}
#endif

#endif /* LICEN_HTTP_H */
