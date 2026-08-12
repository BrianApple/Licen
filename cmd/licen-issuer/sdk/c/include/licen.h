/*
 * licen.h - Licen 授权客户端 SDK（C 语言）
 *
 * 行为与其他语言 SDK 一致（见 docs/protocol.md）：
 *   - licen_start: 注册 + 后台心跳线程（失败不阻塞，进入宽限期）
 *   - 断联宽限期内视为有效；节点被清理自动重新注册（自愈）
 *   - licen_is_valid / licen_has_feature 能力校验
 *
 * 构建：
 *   默认 libcurl 模式（链接 -lcurl -lpthread）
 *   纯 socket 模式（零依赖）：编译时定义 LICEN_NO_CURL=1
 *
 * 示例：
 *     licen_config_t cfg = { .server_url = "http://10.0.0.10:8090",
 *                            .app_key = "hxapigate", .app_secret = "xxx" };
 *     licen_client_t *c = licen_init(&cfg);
 *     licen_start(c);
 *     if (licen_has_feature(c, "ai-inference")) { ... }
 *     licen_stop(c);
 *     licen_destroy(c);
 */
#ifndef LICEN_H
#define LICEN_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
 * 默认产品标识：通用 SDK 为空；厂商定制版（签发服务下载）会把占位符
 * 替换为所选产品 ID，注册/心跳默认携带该产品自声明。
 */
#define LICEN_DEFAULT_PRODUCT "__LICEN_DEFAULT_PRODUCT__"

/* 产品标识（自声明，如 "hxapigate"；NULL 走宽松模式，服务端三层校验） */
#define LICEN_PRODUCT_MAX_LEN 64

/* licen_config 配置项 */
typedef struct licen_config {
    const char *server_url;          /* 授权服务地址，如 http://10.0.0.10:8090（必填） */
    const char *app_key;             /* 应用标识（必填） */
    const char *app_secret;          /* 应用密钥（必填） */
    const char *product;             /* 产品标识（自声明，如 "hxapigate"；NULL 走宽松模式，服务端三层校验） */
    const char *node_name;           /* 节点名称，NULL 用 hostname */
    long heartbeat_interval_sec;     /* 心跳间隔，默认 30 */
    long grace_period_sec;           /* 离线宽限期，默认 300 */
    long connect_timeout_ms;         /* 连接超时，默认 3000 */
} licen_config_t;

/* 授权状态快照 */
#define LICEN_ID_LEN 64
#define LICEN_TIME_LEN 64
#define LICEN_FEATURES_LEN 512

typedef struct licen_status {
    int valid;                       /* 授权是否有效 */
    char license_id[LICEN_ID_LEN];
    char expires_at[LICEN_TIME_LEN];
    int max_nodes;
    int online_nodes;
    char features[LICEN_FEATURES_LEN]; /* 逗号分隔的功能点 */
} licen_status_t;

/* 不透明客户端句柄 */
typedef struct licen_client licen_client_t;

/* 状态变化回调 */
typedef void (*licen_status_callback)(const licen_status_t *status, void *userdata);

/*
 * 创建客户端（不启动）。失败返回 NULL。
 * 成功返回的句柄必须用 licen_destroy 释放。
 */
licen_client_t *licen_init(const licen_config_t *cfg);

/* 启动：注册 + 后台心跳线程。返回 0 成功。 */
int licen_start(licen_client_t *c);

/* 停止心跳线程（不释放句柄） */
void licen_stop(licen_client_t *c);

/* 释放客户端（会先停止） */
void licen_destroy(licen_client_t *c);

/* 是否持有有效授权（最近一次同步状态） */
int licen_is_valid(const licen_client_t *c);

/* 是否拥有指定功能点 */
int licen_has_feature(const licen_client_t *c, const char *feature);

/* 授权中心是否不可达且已过宽限期 */
int licen_is_degraded(const licen_client_t *c);

/* 拷贝当前状态到 out（线程安全） */
void licen_get_status(const licen_client_t *c, licen_status_t *out);

/* 主动刷新状态。返回 0 成功。 */
int licen_refresh(licen_client_t *c);

/* 注册状态变化回调 */
void licen_on_status_change(licen_client_t *c, licen_status_callback cb, void *userdata);

/* 客户端节点 ID（UUID，注册后有效） */
const char *licen_node_id(const licen_client_t *c);

/* 最近一次错误信息（线程局部缓冲） */
const char *licen_last_error(const licen_client_t *c);

#ifdef __cplusplus
}
#endif

#endif /* LICEN_H */
