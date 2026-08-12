/* demo.c - licen-sdk-c 使用示例
 *
 * 运行: ./licen-demo
 * 环境变量: LICEN_SERVER_URL / LICEN_APP_KEY / LICEN_APP_SECRET
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "licen.h"

static void on_change(const licen_status_t *st, void *userdata) {
    (void)userdata;
    printf("📢 授权状态变化: valid=%d licenseId=%s\n", st->valid, st->license_id);
}

int main(void) {
    const char *server_url = getenv("LICEN_SERVER_URL");
    const char *app_key = getenv("LICEN_APP_KEY");
    const char *app_secret = getenv("LICEN_APP_SECRET");

    licen_config_t cfg;
    memset(&cfg, 0, sizeof(cfg));
    cfg.server_url = server_url ? server_url : "http://127.0.0.1:8090";
    cfg.app_key = app_key ? app_key : "hxapigate";
    cfg.app_secret = app_secret ? app_secret : "licen-demo-secret-2026";
    cfg.node_name = "example-c-sdk";
    cfg.heartbeat_interval_sec = 5;

    licen_client_t *c = licen_init(&cfg);
    if (c == NULL) {
        fprintf(stderr, "licen_init 失败\n");
        return 1;
    }

    licen_on_status_change(c, on_change, NULL);
    if (licen_start(c) != 0) {
        fprintf(stderr, "licen_start 失败: %s\n", licen_last_error(c));
        licen_destroy(c);
        return 1;
    }
    printf("🚀 licen-sdk-c 已启动, nodeId=%s\n", licen_node_id(c));
    fflush(stdout);

    for (int i = 0; i < 4; i++) {
        sleep(3);
        licen_status_t st;
        licen_get_status(c, &st);
        printf("状态: valid=%d degraded=%d license=%s 节点=%d/%d 功能=%s\n",
               st.valid, licen_is_degraded(c), st.license_id,
               st.online_nodes, st.max_nodes, st.features);
        fflush(stdout);
    }

    licen_stop(c);
    licen_destroy(c);
    printf("已退出\n");
    return 0;
}
