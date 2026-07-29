# Cursor pagination

[English](../pagination.md) | [简体中文](pagination.md)

`Pager[T]` 提供有界、context-aware cursor iteration，不启动后台 goroutine，也不自动 prefetch。应用把契约
专用 list operation 适配到精简 `PageFetcher[T]` 形状。

## Type

```go
type CursorPage[T any] struct {
	Items      []T
	HasMore    bool
	NextCursor string
}

type PageFetcher[T any] func(
	ctx context.Context,
	cursor string,
) (CursorPage[T], error)

type PagerOptions struct {
	InitialCursor string
	MaxPages      int
}
```

`MaxPages: 0` 使用 `DefaultMaxPages`（10,000）。负值或 nil fetcher 返回
`PaginationInvalidOptions`。

## 最小示例

下面的合成示例展示 pager contract，不调用 live service，也不使用 credential：

```go
package main

import (
	"context"
	"fmt"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func main() {
	pages := map[string]unifyport.CursorPage[string]{
		"": {
			Items:      []string{"first", "second"},
			HasMore:    true,
			NextCursor: "page-2",
		},
		"page-2": {
			Items: []string{"third"},
		},
	}

	pager, err := unifyport.NewPager(
		func(_ context.Context, cursor string) (unifyport.CursorPage[string], error) {
			return pages[cursor], nil
		},
		unifyport.PagerOptions{MaxPages: 3},
	)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	for {
		item, ok, err := pager.Next(ctx)
		if err != nil {
			panic(err)
		}
		if !ok {
			break
		}
		fmt.Println(item)
	}
}
```

适配 SDK list operation 时，fetcher 使用传入 cursor 构造该 operation 的生成 input，调用公开 `Client`
method，再把 response 中 item/`has_more`/`next_cursor` 字段映射到 `CursorPage[T]`。例如 `ListContacts`：

```go
fetcher := func(ctx context.Context, cursor string) (unifyport.CursorPage[unifyport.Contact], error) {
	params := &unifyport.ListContactsParams{}
	if cursor != "" {
		value := unifyport.Cursor(cursor)
		params.Cursor = &value
	}

	result, err := client.ListContacts(ctx, unifyport.ListContactsInput{
		AccountID: accountID,
		Params:    params,
	})
	if err != nil {
		return unifyport.CursorPage[unifyport.Contact]{}, err
	}

	nextCursor := ""
	if result.Data.NextCursor != nil {
		nextCursor = *result.Data.NextCursor
	}
	return unifyport.CursorPage[unifyport.Contact]{
		Items:      result.Data.Items,
		HasMore:    result.Data.HasMore,
		NextCursor: nextCursor,
	}, nil
}
```

完整 [`list-contacts-pager`](../../examples/list-contacts-pager/main.go) 示例从 process environment 读取 account
ID，且不输出 contact record 或 opaque cursor。其他资源可能使用不同 list result 形状；其生成 Go type 和
Device API contract 仍是权威来源。

## 安全不变量

Pager：

- 只有 `Next` 需要下一个 item 时才获取 page；
- 不启动后台 goroutine；
- fetch 前和 fetcher 返回后都检查 `context.Context`；
- 除非 context 已结束，否则原样返回 fetcher error；
- `HasMore` 为 true 时要求非空 next cursor；
- 拒绝已经见过的 cursor，包括重复 `InitialCursor`；
- 到达 `MaxPages` 后停止，即使每页为空且一直声明有更多数据；
- 正常完成后和之后的调用都返回 `ok=false, err=nil`。

一个 `Pager` 只供一个顺序 consumer 使用，不要并发调用 `Next`。

## Error 处理

通过 `errors.As` 获取 `*PaginationError` 并检查 `Reason`：

| Reason | 含义 |
| --- | --- |
| `PaginationInvalidOptions` | Nil fetcher、非法 page limit、nil context 或不可用 pager |
| `PaginationMissingCursor` | `HasMore` 为 true，但 `NextCursor` 为空 |
| `PaginationRepeatedCursor` | Server 返回已使用 cursor |
| `PaginationPageLimit` | 继续请求会超过 `MaxPages` |
| `PaginationCanceled` | Context 被取消 |
| `PaginationTimedOut` | Context deadline 到期 |

`PaginationError` 有意不包含 cursor value，避免 identifier 或 opaque token 进入日志。Canceled/timed-out
pagination 也支持用 `errors.Is` 检查对应 context sentinel。

## Cancellation 责任

Fetcher 必须把 `ctx` 传给 SDK operation。`Pager` 不会创建 goroutine 去中断忽略 context 的 fetcher；这样可以
避免 goroutine leak，但错误实现的 fetcher 仍可能阻塞调用方。
