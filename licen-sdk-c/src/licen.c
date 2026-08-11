/* licen.c - Licen 授权客户端核心实现 */
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif

#include "licen.h"
#include "http.h"
#include "json.h"
#include "sha256.h"

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

/* 内部状态 */
struct licen_client {
    licen_config_t cfg;
    char server_url[256];
    char app_key[128];
    char app_secret[128];
    char node_name[128];
    char node_id[64];

    licen_http_t *http;

    pthread_t thread;
    pthread_mutex_t mutex;
    int started;
    volatile int stop_flag;

    licen_status_t status;
    int degraded;
    long long last_contact_ms;

    licen_status_callback callback;
    void *callback_userdata;

    char error[256];
};

static void set_error(licen_client_t *c, const char *fmt, const char *arg) {
    snprintf(c->error, sizeof(c->error), fmt, arg == NULL ? "" : arg);
}

/* const 指针访问 mutex（pthread 锁需要非 const） */
static pthread_mutex_t *mutex_of(const licen_client_t *c) {
    return (pthread_mutex_t *)&c->mutex;
}

/* 当前毫秒时间戳 */
static long long now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    return (long long)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

/* 生成 UUID v4 */
static void gen_uuid(char *out, size_t out_len) {
    unsigned char b[16];
    FILE *f = fopen("/dev/urandom", "rb");
    if (f != NULL) {
        if (fread(b, 1, 16, f) != 16) {
            memset(b, 0, sizeof(b));
        }
        fclose(f);
    } else {
        srand((unsigned)time(NULL) ^ (unsigned)getpid());
        for (int i = 0; i < 16; i++) {
            b[i] = (unsigned char)(rand() & 0xff);
        }
    }
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    snprintf(out, out_len, "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
             b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
             b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15]);
}

static void get_hostname(char *out, size_t out_len) {
    if (gethostname(out, out_len) != 0) {
        snprintf(out, out_len, "unknown-host");
    }
    out[out_len - 1] = '\0';
}

/* ---------- 状态更新与回调 ---------- */

static void notify_change(licen_client_t *c, licen_status_t *st) {
    if (c->callback != NULL) {
        c->callback(st, c->callback_userdata);
    }
}

static void update_status(licen_client_t *c, const char *json_body) {
    licen_status_t old = c->status;
    licen_status_t nst = c->status;
    char tmp[LICEN_ID_LEN];

    nst.valid = licen_json_get_bool(json_body, "valid", nst.valid);
    if (licen_json_get_string(json_body, "licenseId", tmp, sizeof(tmp)) == 0) {
        snprintf(nst.license_id, sizeof(nst.license_id), "%s", tmp);
    }
    if (licen_json_get_string(json_body, "expiresAt", tmp, sizeof(tmp)) == 0) {
        snprintf(nst.expires_at, sizeof(nst.expires_at), "%s", tmp);
    }
    nst.max_nodes = licen_json_get_int(json_body, "maxNodes", nst.max_nodes);
    nst.online_nodes = licen_json_get_int(json_body, "onlineNodes", nst.online_nodes);
    if (licen_json_get_array(json_body, "features", tmp, sizeof(tmp)) == 0) {
        snprintf(nst.features, sizeof(nst.features), "%s", tmp);
    }

    int changed = old.valid != nst.valid ||
                  strcmp(old.license_id, nst.license_id) != 0;
    c->status = nst;
    if (changed) {
        notify_change(c, &nst);
    }
}

/* 解析注册/心跳响应 */
static int apply_register_response(licen_client_t *c, const char *json_body) {
    if (!licen_json_get_bool(json_body, "success", 0)) {
        char msg[128];
        if (licen_json_get_string(json_body, "message", msg, sizeof(msg)) == 0) {
            set_error(c, "%s", msg);
            return -1;
        }
        set_error(c, "UNKNOWN", NULL);
        return -1;
    }
    licen_status_t nst = c->status;
    char tmp[LICEN_ID_LEN];
    nst.valid = 1;
    if (licen_json_get_string(json_body, "licenseId", tmp, sizeof(tmp)) == 0) {
        snprintf(nst.license_id, sizeof(nst.license_id), "%s", tmp);
    }
    if (licen_json_get_string(json_body, "expiresAt", tmp, sizeof(tmp)) == 0) {
        snprintf(nst.expires_at, sizeof(nst.expires_at), "%s", tmp);
    }
    nst.max_nodes = licen_json_get_int(json_body, "maxNodes", nst.max_nodes);
    nst.online_nodes = licen_json_get_int(json_body, "onlineNodes", nst.online_nodes);
    int changed = c->status.valid != nst.valid || strcmp(c->status.license_id, nst.license_id) != 0;
    c->status = nst;
    if (changed) {
        notify_change(c, &nst);
    }
    return 0;
}

/* ---------- HTTP 请求 ---------- */

static int post_json(licen_client_t *c, const char *path, const char *body, char **out) {
    return licen_http_post(c->http, c->server_url, path, body, out);
}

static int get_json(licen_client_t *c, const char *path, char **out) {
    return licen_http_get(c->http, c->server_url, path, out);
}

/* ---------- 注册 / 心跳 / 刷新 ---------- */

static int do_register(licen_client_t *c) {
    char body[1024];
    char *resp = NULL;
    int rc;
    snprintf(body, sizeof(body),
             "{\"appKey\":\"%s\",\"appSecret\":\"%s\",\"nodeId\":\"%s\",\"nodeName\":\"%s\",\"version\":\"licen-sdk-c-1.0.0\"}",
             c->app_key, c->app_secret, c->node_id, c->node_name);
    rc = post_json(c, "/api/v1/nodes/register", body, &resp);
    if (rc != 0) {
        set_error(c, "HTTP 注册失败", NULL);
        return -1;
    }
    rc = apply_register_response(c, resp);
    licen_http_free_resp(resp);
    if (rc != 0) {
        return -1;
    }
    c->last_contact_ms = now_ms();
    /* 拉取完整状态（含 features） */
    if (get_json(c, "/api/v1/license/status", &resp) == 0) {
        update_status(c, resp);
        licen_http_free_resp(resp);
    }
    return 0;
}

static int do_heartbeat(licen_client_t *c) {
    char body[1024];
    char hmac_str[65];
    char *resp = NULL;
    int rc;
    long long ts = now_ms();
    char msg[256];
    snprintf(msg, sizeof(msg), "%s:%lld", c->node_id, ts);
    licen_hmac_sha256_hex(c->app_secret, msg, hmac_str);
    snprintf(body, sizeof(body),
             "{\"appKey\":\"%s\",\"nodeId\":\"%s\",\"timestamp\":\"%lld\",\"sign\":\"%s\"}",
             c->app_key, c->node_id, ts, hmac_str);
    rc = post_json(c, "/api/v1/nodes/heartbeat", body, &resp);
    if (rc != 0) {
        set_error(c, "HTTP 心跳失败", NULL);
        return -1;
    }
    rc = apply_register_response(c, resp);
    licen_http_free_resp(resp);
    if (rc != 0) {
        return -1;
    }
    c->last_contact_ms = now_ms();
    return 0;
}

static int do_refresh(licen_client_t *c) {
    char *resp = NULL;
    if (get_json(c, "/api/v1/license/status", &resp) != 0) {
        return -1;
    }
    update_status(c, resp);
    licen_http_free_resp(resp);
    return 0;
}

/* 心跳线程 */
static void *heartbeat_loop(void *arg) {
    licen_client_t *c = (licen_client_t *)arg;
    long interval_ms = c->cfg.heartbeat_interval_sec > 0
                           ? c->cfg.heartbeat_interval_sec * 1000
                           : 30 * 1000;
    long grace_ms = c->cfg.grace_period_sec > 0 ? c->cfg.grace_period_sec * 1000 : 300 * 1000;
    while (!c->stop_flag) {
        usleep((useconds_t)interval_ms * 1000);
        if (c->stop_flag) {
            break;
        }
        if (do_heartbeat(c) != 0) {
            /* 节点被清理/服务重启 → 自动重新注册（自愈） */
            if (strstr(c->error, "NODE_NOT_FOUND") != NULL) {
                if (do_register(c) == 0) {
                    c->degraded = 0;
                    continue;
                }
            }
            if (now_ms() - c->last_contact_ms > grace_ms) {
                pthread_mutex_lock(mutex_of(c));
                c->degraded = 1;
                pthread_mutex_unlock(mutex_of(c));
                fprintf(stderr, "[licen-sdk-c] ⚠️ 授权中心不可达，进入宽限期: %s\n", c->error);
            }
        } else {
            pthread_mutex_lock(mutex_of(c));
            c->degraded = 0;
            pthread_mutex_unlock(mutex_of(c));
        }
    }
    return NULL;
}

/* ---------- 公开 API ---------- */

licen_client_t *licen_init(const licen_config_t *cfg) {
    if (cfg == NULL || cfg->server_url == NULL || cfg->app_key == NULL || cfg->app_secret == NULL) {
        return NULL;
    }
    licen_client_t *c = (licen_client_t *)calloc(1, sizeof(*c));
    if (c == NULL) {
        return NULL;
    }
    if (cfg->heartbeat_interval_sec > 0) {
        c->cfg.heartbeat_interval_sec = cfg->heartbeat_interval_sec;
    } else {
        c->cfg.heartbeat_interval_sec = 30;
    }
    if (cfg->grace_period_sec > 0) {
        c->cfg.grace_period_sec = cfg->grace_period_sec;
    } else {
        c->cfg.grace_period_sec = 300;
    }
    c->cfg.connect_timeout_ms = cfg->connect_timeout_ms > 0 ? cfg->connect_timeout_ms : 3000;

    snprintf(c->server_url, sizeof(c->server_url), "%s", cfg->server_url);
    snprintf(c->app_key, sizeof(c->app_key), "%s", cfg->app_key);
    snprintf(c->app_secret, sizeof(c->app_secret), "%s", cfg->app_secret);
    if (cfg->node_name != NULL && cfg->node_name[0] != '\0') {
        snprintf(c->node_name, sizeof(c->node_name), "%s", cfg->node_name);
    } else {
        get_hostname(c->node_name, sizeof(c->node_name));
    }
    gen_uuid(c->node_id, sizeof(c->node_id));

    pthread_mutex_init(&c->mutex, NULL);
    c->http = licen_http_new(c->cfg.connect_timeout_ms);
    if (c->http == NULL) {
        pthread_mutex_destroy(&c->mutex);
        free(c);
        return NULL;
    }
    return c;
}

int licen_start(licen_client_t *c) {
    if (c == NULL) {
        return -1;
    }
    pthread_mutex_lock(mutex_of(c));
    if (c->started) {
        pthread_mutex_unlock(mutex_of(c));
        return 0;
    }
    c->started = 1;
    c->stop_flag = 0;
    pthread_mutex_unlock(mutex_of(c));

    /* 首次注册（失败不阻断启动，进入宽限期重试） */
    if (do_register(c) != 0) {
        c->degraded = 1;
    }
    if (pthread_create(&c->thread, NULL, heartbeat_loop, c) != 0) {
        pthread_mutex_lock(mutex_of(c));
        c->started = 0;
        pthread_mutex_unlock(mutex_of(c));
        return -1;
    }
    return 0;
}

void licen_stop(licen_client_t *c) {
    if (c == NULL) {
        return;
    }
    c->stop_flag = 1;
    if (c->started && c->thread != 0) {
        pthread_join(c->thread, NULL);
        c->thread = 0;
        c->started = 0;
    }
}

void licen_destroy(licen_client_t *c) {
    if (c == NULL) {
        return;
    }
    licen_stop(c);
    if (c->http != NULL) {
        licen_http_free(c->http);
    }
    pthread_mutex_destroy(&c->mutex);
    free(c);
}

int licen_is_valid(const licen_client_t *c) {
    if (c == NULL) {
        return 0;
    }
    pthread_mutex_lock(mutex_of(c));
    int valid = c->status.valid;
    pthread_mutex_unlock(mutex_of(c));
    return valid;
}

int licen_has_feature(const licen_client_t *c, const char *feature) {
    if (c == NULL || feature == NULL) {
        return 0;
    }
    pthread_mutex_lock(mutex_of(c));
    int found = 0;
    if (c->status.valid && c->status.features[0] != '\0') {
        char copy[LICEN_FEATURES_LEN];
        snprintf(copy, sizeof(copy), "%s", c->status.features);
        char *save = NULL;
        char *tok = strtok_r(copy, ",", &save);
        while (tok != NULL) {
            while (*tok == ' ') {
                tok++;
            }
            if (strcmp(tok, feature) == 0) {
                found = 1;
                break;
            }
            tok = strtok_r(NULL, ",", &save);
        }
    }
    pthread_mutex_unlock(mutex_of(c));
    return found;
}

int licen_is_degraded(const licen_client_t *c) {
    if (c == NULL) {
        return 1;
    }
    pthread_mutex_lock(mutex_of(c));
    int d = c->degraded;
    pthread_mutex_unlock(mutex_of(c));
    return d;
}

void licen_get_status(const licen_client_t *c, licen_status_t *out) {
    if (c == NULL || out == NULL) {
        return;
    }
    pthread_mutex_lock(mutex_of(c));
    *out = c->status;
    pthread_mutex_unlock(mutex_of(c));
}

int licen_refresh(licen_client_t *c) {
    if (c == NULL) {
        return -1;
    }
    return do_refresh(c);
}

void licen_on_status_change(licen_client_t *c, licen_status_callback cb, void *userdata) {
    if (c == NULL) {
        return;
    }
    pthread_mutex_lock(mutex_of(c));
    c->callback = cb;
    c->callback_userdata = userdata;
    pthread_mutex_unlock(mutex_of(c));
}

const char *licen_node_id(const licen_client_t *c) {
    return c == NULL ? "" : c->node_id;
}

const char *licen_last_error(const licen_client_t *c) {
    return c == NULL ? "" : c->error;
}
