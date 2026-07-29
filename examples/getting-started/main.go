package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func main() {
	baseURL := requiredEnv("UNIFYPORT_DEVICE_API_BASE_URL")
	apiKey := requiredEnv("UNIFYPORT_DEVICE_API_KEY")

	// 凭据只从运行环境进入 provider，避免写入源码、示例输出或 Client 的公开字段。
	client, err := unifyport.NewClient(unifyport.Config{
		BaseURL:        baseURL,
		APIKeyProvider: unifyport.StaticAPIKey(apiKey),
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := client.GetWorkspace(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// 只输出经过过滤的诊断元数据，业务 data 可能包含不适合日志记录的信息。
	fmt.Printf("status=%d request_id=%s client_request_id=%s\n",
		result.StatusCode,
		result.RequestID,
		result.ClientRequestID,
	)
}

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		log.Fatalf("missing required environment variable: %s", name)
	}
	return value
}
