/* http_curl.c - libcurl 实现的 HTTP 传输（默认模式） */
#include "http.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef LICEN_NO_CURL
/* 纯 socket 模式编译本文件为空实现（链接 http_socket.c） */
licen_http_t *licen_http_new(long connect_timeout_ms) { (void)connect_timeout_ms; return NULL; }
void licen_http_free(licen_http_t *h) { (void)h; }
int licen_http_post(licen_http_t *h, const char *url, const char *path, const char *body, char **out) {
    (void)h; (void)url; (void)path; (void)body; (void)out; return -1;
}
int licen_http_get(licen_http_t *h, const char *url, const char *path, char **out) {
    (void)h; (void)url; (void)path; (void)out; return -1;
}
void licen_http_free_resp(char *resp) { (void)resp; }
#else

#include <curl/curl.h>

struct licen_http {
    long timeout_ms;
};

static size_t write_cb(void *ptr, size_t size, size_t nmemb, void *userdata) {
    size_t total = size * nmemb;
    char **buf = (char **)userdata;
    size_t old_len = *buf == NULL ? 0 : strlen(*buf);
    char *nb = realloc(*buf, old_len + total + 1);
    if (nb == NULL) {
        return 0;
    }
    memcpy(nb + old_len, ptr, total);
    nb[old_len + total] = '\0';
    *buf = nb;
    return total;
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

static int do_request(licen_http_t *h, const char *url, const char *path,
                      const char *body, char **out) {
    CURL *curl = curl_easy_init();
    char *resp = NULL;
    CURLcode rc;
    long http_code = 0;
    char full_url[1024];

    if (curl == NULL) {
        return -1;
    }
    snprintf(full_url, sizeof(full_url), "%s%s", url, path);
    curl_easy_setopt(curl, CURLOPT_URL, full_url);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT_MS, h->timeout_ms);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT_MS, h->timeout_ms);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &resp);
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER,
                     curl_slist_append(NULL, "Content-Type: application/json"));
    if (body != NULL) {
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body);
    }
    rc = curl_easy_perform(curl);
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);
    curl_easy_cleanup(curl);
    if (rc != CURLE_OK) {
        free(resp);
        return -1;
    }
    if (http_code >= 400) {
        free(resp);
        return -1;
    }
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

#endif /* LICEN_NO_CURL */
