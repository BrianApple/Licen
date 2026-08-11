package com.licen.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.licen.sdk.crypto.HmacUtil;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Licen 客户端：向授权服务注册、心跳保活，缓存授权状态供业务校验。
 *
 * <p>用法（Spring 环境由 LicenAutoConfiguration 自动装配，直接注入即可）：</p>
 * <pre>
 * &#64;Autowired LicenClient licenClient;
 * if (licenClient.isLicenseValid()) { ... }
 * if (licenClient.hasFeature("ai-inference")) { ... }
 * </pre>
 */
public class LicenClient {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final LicenSdkProperties properties;
    private final String nodeId;
    private final String nodeName;
    private final HttpClient httpClient;
    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor(r -> {
        Thread t = new Thread(r, "licen-sdk-heartbeat");
        t.setDaemon(true);
        return t;
    });

    private final AtomicBoolean started = new AtomicBoolean(false);
    private final AtomicBoolean degraded = new AtomicBoolean(false);
    private final AtomicReference<LicenseStatus> status = new AtomicReference<>(LicenseStatus.unknown());

    /** 最近一次心跳成功时间 */
    private volatile long lastContactAt = 0;

    public LicenClient(LicenSdkProperties properties) {
        this.properties = properties;
        String hostname = safeHostname();
        this.nodeName = properties.getNodeName() == null || properties.getNodeName().isBlank()
                ? hostname : properties.getNodeName();
        this.nodeId = UUID.randomUUID().toString().replace("-", "");
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofMillis(properties.getConnectTimeoutMs()))
                .build();
    }

    /** 启动：注册 + 定时心跳（应用启动时调用） */
    public synchronized void start() {
        if (!properties.isEnabled() || !started.compareAndSet(false, true)) {
            return;
        }
        // 首次注册（失败不阻断启动，进入宽限期重试）
        try {
            register();
        } catch (Exception e) {
            markDegraded("注册失败: " + e.getMessage());
        }
        scheduler.scheduleWithFixedDelay(this::heartbeatSafe,
                properties.getHeartbeatIntervalSeconds(),
                properties.getHeartbeatIntervalSeconds(),
                TimeUnit.SECONDS);
    }

    /** 停止心跳线程（应用关闭时调用） */
    public void stop() {
        started.set(false);
        scheduler.shutdownNow();
    }

    /** 是否持有有效授权（最近一次从授权中心同步的状态） */
    public boolean isLicenseValid() {
        return status.get().valid;
    }

    /** 是否拥有指定功能点 */
    public boolean hasFeature(String feature) {
        LicenseStatus s = status.get();
        return s.valid && s.features != null && s.features.contains(feature);
    }

    /** 授权中心是否可达且最近心跳正常（false = 降级/离线） */
    public boolean isDegraded() {
        return degraded.get();
    }

    public LicenseStatus getStatus() {
        return status.get();
    }

    public String getNodeId() {
        return nodeId;
    }

    // ---------- 内部实现 ----------

    private void register() throws Exception {
        String body = MAPPER.writeValueAsString(java.util.Map.of(
                "appKey", nz(properties.getAppKey()),
                "appSecret", nz(properties.getAppSecret()),
                "nodeId", nodeId,
                "nodeName", nodeName,
                "ip", localIp(),
                "version", "licen-sdk-1.0.0"));
        JsonNode json = post("/api/v1/nodes/register", body, null);
        applyRegisterResponse(json);
        lastContactAt = System.currentTimeMillis();
        degraded.set(false);
        // 拉取完整授权状态（含 features 功能点）
        refreshStatus();
    }

    private void heartbeatSafe() {
        try {
            heartbeat();
        } catch (Exception e) {
            // 节点已被清理/授权服务重启 → 自动重新注册（自愈）
            if (e.getMessage() != null && e.getMessage().contains("NODE_NOT_FOUND")) {
                try {
                    register();
                    return;
                } catch (Exception ignored) {
                    // 注册失败继续走降级逻辑
                }
            }
            long silentMs = System.currentTimeMillis() - lastContactAt;
            if (silentMs > properties.getGracePeriodSeconds() * 1000L) {
                markDegraded("心跳连续失败: " + e.getMessage());
            }
        }
    }

    private void heartbeat() throws Exception {
        long ts = System.currentTimeMillis();
        String sign = HmacUtil.hmacSha256Hex(properties.getAppSecret(), nodeId + ":" + ts);
        String body = MAPPER.writeValueAsString(java.util.Map.of(
                "appKey", nz(properties.getAppKey()),
                "nodeId", nodeId,
                "timestamp", String.valueOf(ts),
                "sign", sign));
        JsonNode json = post("/api/v1/nodes/heartbeat", body, null);
        if (json.path("success").asBoolean(false)) {
            lastContactAt = System.currentTimeMillis();
            degraded.set(false);
            updateStatus(json);
        } else {
            throw new IllegalStateException("心跳被拒绝: " + json.path("message").asText());
        }
    }

    /** 主动向授权中心查询最新状态 */
    public void refreshStatus() {
        try {
            JsonNode json = get("/api/v1/license/status");
            updateStatus(json);
        } catch (Exception ignored) {
            // 查询失败保持本地缓存
        }
    }

    private void applyRegisterResponse(JsonNode json) {
        if (!json.path("success").asBoolean(false)) {
            throw new IllegalStateException("注册被拒绝: " + json.path("message").asText());
        }
        updateStatus(json);
    }

    private void updateStatus(JsonNode json) {
        LicenseStatus s = new LicenseStatus();
        s.valid = json.path("valid").asBoolean(json.path("success").asBoolean(false));
        s.licenseId = textOrNull(json, "licenseId");
        s.expiresAt = textOrNull(json, "expiresAt");
        s.maxNodes = json.path("maxNodes").asInt(-1);
        s.onlineNodes = json.path("onlineNodes").asInt(-1);
        if (json.has("features") && json.get("features").isArray()) {
            List<String> fs = new java.util.ArrayList<>();
            json.get("features").forEach(f -> fs.add(f.asText()));
            s.features = fs;
        }
        s.lastSyncAt = System.currentTimeMillis();
        status.set(s);
    }

    private JsonNode post(String path, String body, String signHeader) throws Exception {
        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(properties.getServerUrl() + path))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8));
        if (signHeader != null) {
            builder.header("X-Licen-Sign", signHeader);
        }
        HttpResponse<String> resp = httpClient.send(builder.build(),
                HttpResponse.BodyHandlers.ofString());
        if (resp.statusCode() >= 400) {
            throw new IllegalStateException("HTTP " + resp.statusCode() + ": " + resp.body());
        }
        return MAPPER.readTree(resp.body());
    }

    private JsonNode get(String path) throws Exception {
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(properties.getServerUrl() + path))
                .GET()
                .build();
        HttpResponse<String> resp = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        if (resp.statusCode() >= 400) {
            throw new IllegalStateException("HTTP " + resp.statusCode() + ": " + resp.body());
        }
        return MAPPER.readTree(resp.body());
    }

    private void markDegraded(String reason) {
        degraded.set(true);
        System.err.println("[licen-sdk] ⚠️ 授权中心不可达，进入宽限期: " + reason);
    }

    private static String nz(String s) {
        return s == null ? "" : s;
    }

    private static String textOrNull(JsonNode json, String field) {
        JsonNode v = json.get(field);
        return v == null || v.isNull() ? null : v.asText();
    }

    private static String safeHostname() {
        try {
            return java.net.InetAddress.getLocalHost().getHostName();
        } catch (Exception e) {
            return "unknown-host";
        }
    }

    private static String localIp() {
        try {
            return java.net.InetAddress.getLocalHost().getHostAddress();
        } catch (Exception e) {
            return "";
        }
    }

    /** 授权状态快照 */
    public static class LicenseStatus {
        public boolean valid;
        public String licenseId;
        public String expiresAt;
        public int maxNodes = -1;
        public int onlineNodes = -1;
        public List<String> features;
        public long lastSyncAt;

        public static LicenseStatus unknown() {
            return new LicenseStatus();
        }
    }
}
