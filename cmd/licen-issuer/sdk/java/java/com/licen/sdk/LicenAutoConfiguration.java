package com.licen.sdk;

import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Licen SDK 自动配置。
 *
 * <p>引入依赖后，配置 licen.sdk.* 即自动注册 LicenClient 并启动心跳。</p>
 */
@AutoConfiguration
@EnableConfigurationProperties(LicenSdkProperties.class)
@ConditionalOnProperty(prefix = "licen.sdk", name = "enabled", havingValue = "true", matchIfMissing = true)
public class LicenAutoConfiguration {

    @Bean
    @ConditionalOnMissingBean
    public LicenClient licenClient(LicenSdkProperties properties) {
        return new LicenClient(properties);
    }

    @Bean
    @ConditionalOnMissingBean
    public LicenLifecycle licenLifecycle(LicenClient licenClient) {
        return new LicenLifecycle(licenClient);
    }

    /** 应用生命周期管理：启动注册心跳，关闭停止 */
    @Configuration
    public static class LicenLifecycle implements org.springframework.context.SmartLifecycle {

        private final LicenClient licenClient;
        private volatile boolean running = false;

        public LicenLifecycle(LicenClient licenClient) {
            this.licenClient = licenClient;
        }

        @Override
        public void start() {
            licenClient.start();
            running = true;
        }

        @Override
        public void stop() {
            licenClient.stop();
            running = false;
        }

        @Override
        public boolean isRunning() {
            return running;
        }
    }
}
