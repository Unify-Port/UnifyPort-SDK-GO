package unifyport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestIncomingCallLiteralFieldsRemainTyped(t *testing.T) {
	var event CallIncomingWebhookEvent
	if err := json.Unmarshal([]byte(`{"data":{"call":{"id":"call_example","direction":"inbound","media_type":"voice","state":"ringing"},"sender":{"id":"peer_example"},"join_url":"https://calls.example.com/example"}}`), &event); err != nil {
		t.Fatal(err)
	}
	// const 不能退化为 interface{}；使用生成枚举才能识别不属于该声明的方向或状态。
	if !event.Data.Call.Direction.Valid() || !event.Data.Call.State.Valid() {
		t.Fatal("incoming call literals were lost")
	}
	event.Data.Call.Direction = "outbound"
	if event.Data.Call.Direction.Valid() {
		t.Fatal("incoming-call direction was widened")
	}
}

func TestDeleteAccountPreservesAcceptedOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Path != "/v1/accounts/acc_example" {
			t.Errorf("wrong delete request")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(writer, `{"data":{"operation_id":"op_example","kind":"DELETE","state":"PENDING"}}`)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKeyProvider: StaticAPIKey("test-key"), HTTPClient: server.Client(), AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.DeleteAccount(context.Background(), DeleteAccountInput{AccountID: "acc_example"})
	// 202 仅表示接收操作；必须保留操作 ID 和状态，不能转换为 NoContent 或成功完成。
	if err != nil || result.StatusCode != 202 || result.Data == nil || result.Data.OperationID != "op_example" || result.Data.State != "PENDING" {
		t.Fatalf("accepted deletion: %#v %v", result, err)
	}
}

func TestIdempotentAccountCreationDoesNotEnableAutomaticRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts.Add(1)
		if request.Header.Get("Idempotency-Key") != "idem_example" || request.Header.Get("X-Api-Key") != "test-key" {
			t.Error("creation headers were not preserved")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(503)
		_, _ = io.WriteString(writer, `{"error":{"code":"wallet_unavailable","numeric_code":"20028"}}`)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKeyProvider: StaticAPIKey("test-key"), HTTPClient: server.Client(), AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	key := "idem_example"
	_, err = client.CreateAccount(context.Background(), CreateAccountInput{Params: &CreateAccountParams{IdempotencyKey: &key}, Body: CreateAccountRequestBody{Provider: "whatsapp", Region: "example", AuthMode: "qrcode"}})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.NumericCode != "20028" || attempts.Load() != 1 {
		t.Fatalf("creation retry policy: %#v attempts=%d", err, attempts.Load())
	}
}
