// licen-server：私有化部署产品授权服务（Go 单二进制）。
//
// 构建（生产，公钥内嵌 + 去符号）：
//
//	go build -ldflags "-s -w -X main.publicKey=<公钥Base64>" -o licen-server ./cmd/licen-server
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BrianApple/Licen/internal/api"
	"github.com/BrianApple/Licen/internal/config"
	"github.com/BrianApple/Licen/internal/license"
	"github.com/BrianApple/Licen/internal/node"
	"github.com/BrianApple/Licen/internal/store"
)

// publicKey 构建时注入的默认公钥（Base64，X.509）。
// 部署时可用 ./keys/public.pem 文件覆盖（文件优先）。
var publicKey = ""

func main() {
	configPath := flag.String("c", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}

	// 1. 存储
	st, err := store.Open(cfg.Licen.DBPath)
	if err != nil {
		slog.Error("存储初始化失败", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// 2. 授权管理（公钥：外部文件优先，否则用构建注入值）
	pubB64 := publicKey
	if data, err := os.ReadFile(cfg.Licen.PublicKeyFile); err == nil {
		pubB64 = string(data)
		slog.Info("🔐 使用外部公钥文件", "file", cfg.Licen.PublicKeyFile)
	} else if publicKey != "" {
		slog.Info("🔐 使用构建内嵌公钥")
	} else {
		slog.Error("未提供公钥：请放置 keys/public.pem 或构建时 -ldflags -X main.publicKey=...")
		os.Exit(1)
	}
	licMgr, err := license.NewManager(cfg.Licen.Salt, cfg.Licen.LicenseFile, pubB64)
	if err != nil {
		slog.Error("授权管理器初始化失败", "err", err)
		os.Exit(1)
	}
	licMgr.Reload()

	// 3. 节点服务 + 回收协程
	nodeSvc := node.New(st, licMgr, cfg.Licen.HeartbeatTimeoutSeconds, cfg.Licen.HmacVerifyEnabled)
	stopCh := make(chan struct{})
	defer close(stopCh)
	go nodeSvc.RecycleLoop(stopCh)

	// 4. 默认应用（开箱即用，生产务必修改）
	bootstrapDefaultApp(st)

	// 5. HTTP 服务
	apiSrv := api.New(st, licMgr, nodeSvc, cfg.Licen.AdminToken)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: apiSrv.Handler(),
	}
	go func() {
		slog.Info("🚀 licen-server 启动", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	// 6. 优雅停机
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("🛑 正在停止...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

// bootstrapDefaultApp 无应用时创建默认示例应用
func bootstrapDefaultApp(st *store.Store) {
	count, err := st.AppCount()
	if err != nil || count > 0 {
		return
	}
	app := &store.App{
		AppKey:    "hxapigate",
		AppSecret: "licen-demo-secret-2026",
		Product:   "hxapigate",
		Name:      "HXAPIGate智能网关（默认示例应用）",
		Enabled:   true,
	}
	if err := st.CreateApp(app); err == nil {
		slog.Warn("🛠️ 已创建默认示例应用: appKey=hxapigate appSecret=licen-demo-secret-2026（生产环境请务必修改！）")
	}
}
