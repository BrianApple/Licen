/* http_socket.c - 纯 POSIX socket 实现的 HTTP/1.1 客户端（零依赖模式，LICEN_NO_CURL=1）
 *
 * 说明：
 *   - 仅支持 http://（无 TLS，内网部署场景）
 *   - 仅支持固定响应体（Content-Length），不支持 chunked（licen-server 响应均为 Content-Length）
 */
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200112L
#endif

#include "http.h"

#include <errno.h>
#include <netdb.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include <sys/socket.h>
#include <sys/time.h>
#include <sys/types.h>

struct licen_http {
    long timeout_ms;
};

/* 解析 URL: http://host[:port] 返回 host/port（请求路径统一由 path 参数提供） */
static int parse_url(const char *url, char *host, size_t host_len, int *port) {
    const char *p = url;
    if (strncmp(p, "http://", 7) != 0) {
        return -1;
    }
    p += 7;
    const char *host_start = p;
    while (*p && *p != ':' && *p != '/') {
        p++;
    }
    size_t hlen = (size_t)(p - host_start);
    if (hlen == 0 || hlen >= host_len) {
        return -1;
    }
    memcpy(host, host_start, hlen);
    host[hlen] = '\0';
    *port = 80;
    if (*p == ':') {
        *port = atoi(p + 1);
    }
    return 0;
}

licen_http_t *licen_http_new(long connect_timeout_ms) {
    licen_http_t *h = (licen_http_t *)calloc(1, sizeof(*h));
    if (h == NULL) {
        return NULL;
    }
    h->timeout_ms = connect_timeout_ms > 0 ? connect_timeout_ms : 3000;
    return h;
}

void licen_http_free(licen_http_t *h) { free(h); }

static int connect_socket(const char *host, int port, long timeout_ms) {
    struct addrinfo hints, *res = NULL;
    char portstr[16];
    int fd = -1;
    struct timeval tv;

    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    snprintf(portstr, sizeof(portstr), "%d", port);
    if (getaddrinfo(host, portstr, &hints, &res) != 0) {
        return -1;
    }
    for (struct addrinfo *ai = res; ai != NULL; ai = ai->ai_next) {
        fd = socket(ai->ai_family, ai->ai_socktype, ai->ai_protocol);
        if (fd < 0) {
            continue;
        }
        if (connect(fd, ai->ai_addr, ai->ai_addrlen) == 0) {
            break;
        }
        close(fd);
        fd = -1;
    }
    freeaddrinfo(res);
    if (fd < 0) {
        return -1;
    }
    tv.tv_sec = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
    return fd;
}

static int recv_all(int fd, char *buf, size_t len) {
    size_t got = 0;
    while (got < len) {
        ssize_t n = recv(fd, buf + got, len - got, 0);
        if (n <= 0) {
            return -1;
        }
        got += (size_t)n;
    }
    buf[len] = '\0';
    return 0;
}

static int do_request(licen_http_t *h, const char *url, const char *path,
                      const char *body, char **out) {
    char host[256];
    int port;
    int fd;
    char req[4096];
    char header[4096];
    char *resp = NULL;
    size_t header_len;
    const char *cl;
    long content_length = 0;
    char *body_start;

    if (parse_url(url, host, sizeof(host), &port) != 0) {
        return -1;
    }
    fd = connect_socket(host, port, h->timeout_ms);
    if (fd < 0) {
        return -1;
    }

    if (body != NULL) {
        snprintf(req, sizeof(req),
                 "POST %s HTTP/1.1\r\n"
                 "Host: %s:%d\r\n"
                 "Content-Type: application/json\r\n"
                 "Content-Length: %zu\r\n"
                 "Connection: close\r\n\r\n%s",
                 path, host, port, strlen(body), body);
    } else {
        snprintf(req, sizeof(req),
                 "GET %s HTTP/1.1\r\n"
                 "Host: %s:%d\r\n"
                 "Connection: close\r\n\r\n",
                 path, host, port);
    }
    if (write(fd, req, strlen(req)) < 0) {
        close(fd);
        return -1;
    }

    /* 读响应头 */
    header_len = 0;
    while (header_len + 1 < sizeof(header)) {
        ssize_t n = recv(fd, header + header_len, 1, 0);
        if (n <= 0) {
            close(fd);
            return -1;
        }
        header_len += (size_t)n;
        if (header_len >= 4 && memcmp(header + header_len - 4, "\r\n\r\n", 4) == 0) {
            break;
        }
    }
    header[header_len] = '\0';
    if (header_len < 12 || strncmp(header, "HTTP/1.1 ", 9) != 0) {
        close(fd);
        return -1;
    }
    if (strncmp(header + 9, "200", 3) != 0 && strncmp(header + 9, "201", 3) != 0) {
        close(fd);
        return -1;
    }
    cl = strstr(header, "Content-Length:");
    if (cl == NULL) {
        close(fd);
        return -1;
    }
    content_length = atol(cl + 15);
    if (content_length < 0 || content_length > 10 * 1024 * 1024) {
        close(fd);
        return -1;
    }
    body_start = strstr(header, "\r\n\r\n");
    if (body_start == NULL) {
        close(fd);
        return -1;
    }
    body_start += 4;

    resp = (char *)malloc((size_t)content_length + 1);
    if (resp == NULL) {
        close(fd);
        return -1;
    }
    if (recv_all(fd, resp, (size_t)content_length) != 0) {
        free(resp);
        close(fd);
        return -1;
    }
    close(fd);
    *out = resp;
    return 0;
}

int licen_http_post(licen_http_t *h, const char *url, const char *path,
                    const char *json_body, char **out) {
    return do_request(h, url, path, json_body, out);
}

int licen_http_get(licen_http_t *h, const char *url, const char *path, char **out) {
    return do_request(h, url, path, NULL, out);
}

void licen_http_free_resp(char *resp) { free(resp); }
