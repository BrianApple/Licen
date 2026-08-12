package com.licen.sdk;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * licen-sdk 客户端配置。
 *
 * <pre>
 * licen.sdk.server-url: http://10.0.0.10:8090   # 授权服务地址（客户VM）
 * licen.sdk.app-key: hxapigate
 * licen.sdk.app-secret: xxx
 * licen.sdk.product: hxapigate                  # 产品标识（自声明，须与管理端 App 绑定产品一致）
 * licen.sdk.node-name: hxapigate-1              # 可选，默认取 hostname
 * licen.sdk.heartbeat-interval-seconds: 30
 * licen.sdk.grace-period-seconds: 300           # 授权中心不可达时的宽限期
 * </pre>
 */
@ConfigurationProperties(prefix = "licen.sdk")
public class LicenSdkProperties {

    /**
     * 默认产品标识：通用 SDK 为空；厂商定制版（签发服务下载）会把占位符
     * 替换为所选产品 ID，注册/心跳默认携带该产品自声明。
     */
    public static final String DEFAULT_PRODUCT = "__LICEN_DEFAULT_PRODUCT__";

    /** 授权服务地址，如 http://10.0.0.10:8090 */
    private String serverUrl = "http://127.0.0.1:8090";

    /** 应用标识（与授权服务创建的应用一致） */
    private String appKey;

    /** 应用密钥 */
    private String appSecret;

    /** 产品标识（自声明，如 hxapigate；服务端三层校验：SDK 声明 == App 凭证 == License 签名授权） */
    private String product = DEFAULT_PRODUCT;

    /** 节点名称（可选，默认本机 hostname） */
    private String nodeName;

    /** 心跳间隔（秒） */
    private long heartbeatIntervalSeconds = 30;

    /** 授权中心不可达宽限期（秒）：超过后判定为降级，业务可据此限制能力 */
    private long gracePeriodSeconds = 300;

    /** 连接超时（毫秒） */
    private long connectTimeoutMs = 3000;

    /** 是否启用（false 则 SDK 不注册不心跳，仅本地直连校验） */
    private boolean enabled = true;

    public String getServerUrl() {
        return serverUrl;
    }

    public void setServerUrl(String serverUrl) {
        this.serverUrl = serverUrl;
    }

    public String getAppKey() {
        return appKey;
    }

    public void setAppKey(String appKey) {
        this.appKey = appKey;
    }

    public String getAppSecret() {
        return appSecret;
    }

    public void setAppSecret(String appSecret) {
        this.appSecret = appSecret;
    }

    public String getProduct() {
        return product;
    }

    public void setProduct(String product) {
        this.product = product;
    }

    public String getNodeName() {
        return nodeName;
    }

    public void setNodeName(String nodeName) {
        this.nodeName = nodeName;
    }

    public long getHeartbeatIntervalSeconds() {
        return heartbeatIntervalSeconds;
    }

    public void setHeartbeatIntervalSeconds(long heartbeatIntervalSeconds) {
        this.heartbeatIntervalSeconds = heartbeatIntervalSeconds;
    }

    public long getGracePeriodSeconds() {
        return gracePeriodSeconds;
    }

    public void setGracePeriodSeconds(long gracePeriodSeconds) {
        this.gracePeriodSeconds = gracePeriodSeconds;
    }

    public long getConnectTimeoutMs() {
        return connectTimeoutMs;
    }

    public void setConnectTimeoutMs(long connectTimeoutMs) {
        this.connectTimeoutMs = connectTimeoutMs;
    }

    public boolean isEnabled() {
        return enabled;
    }

    public void setEnabled(boolean enabled) {
        this.enabled = enabled;
    }
}
