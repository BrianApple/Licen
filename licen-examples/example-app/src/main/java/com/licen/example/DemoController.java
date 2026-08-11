package com.licen.example;

import com.licen.sdk.LicenClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 演示业务侧如何基于授权状态控制产品能力。
 */
@RestController
@RequestMapping("/demo")
public class DemoController {

    private final LicenClient licenClient;

    public DemoController(LicenClient licenClient) {
        this.licenClient = licenClient;
    }

    /** 授权状态总览 */
    @GetMapping("/check")
    public Map<String, Object> check() {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("licenseValid", licenClient.isLicenseValid());
        body.put("degraded", licenClient.isDegraded());
        body.put("nodeId", licenClient.getNodeId());
        LicenClient.LicenseStatus status = licenClient.getStatus();
        body.put("licenseId", status.licenseId);
        body.put("expiresAt", status.expiresAt);
        body.put("onlineNodes", status.onlineNodes);
        body.put("maxNodes", status.maxNodes);
        body.put("features", status.features);
        return body;
    }

    /** 业务能力开关示例：AI 推理功能是否可用 */
    @GetMapping("/feature/{name}")
    public Map<String, Object> feature(@PathVariable String name) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("feature", name);
        body.put("enabled", licenClient.hasFeature(name));
        return body;
    }
}
