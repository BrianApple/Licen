package com.licen.example;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * 示例应用：展示被授权产品如何接入 licen-sdk。
 *
 * <p>只需引入 licen-sdk 依赖并配置 licen.sdk.* 即可自动注册 + 心跳，
 * 业务代码注入 LicenClient 进行能力校验。</p>
 */
@SpringBootApplication
public class ExampleApp {

    public static void main(String[] args) {
        SpringApplication.run(ExampleApp.class, args);
    }
}
