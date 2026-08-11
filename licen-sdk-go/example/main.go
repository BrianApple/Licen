// licen-sdk-go 使用示例。
//
// 运行：go run ./example/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BrianApple/Licen/licen-sdk-go"
)

func main() {
	client, err := licen.NewClient(licen.Config{
		ServerURL: env("LICEN_SERVER_URL", "http://127.0.0.1:8090"),
		AppKey:    env("LICEN_APP_KEY", "ai-engine"),
		AppSecret: env("LICEN_APP_SECRET", "licen-demo-secret-2026"),
		NodeName:  "example-go-sdk",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 状态变化回调（可选）
	client.OnStatusChange(func(st licen.Status) {
		fmt.Printf("📢 授权状态变化: valid=%v licenseId=%s\n", st.Valid, st.LicenseID)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.Start(ctx)
	fmt.Println("🚀 licen-sdk-go 已启动，nodeId:", client.NodeID())

	// 每秒打印一次授权状态
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			st := client.Status()
			fmt.Printf("状态: valid=%v degraded=%v license=%s 节点=%d/%d 功能=%v\n",
				st.Valid, client.IsDegraded(), st.LicenseID,
				st.OnlineNodes, st.MaxNodes, st.Features)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	client.Stop()
	fmt.Println("已退出")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
