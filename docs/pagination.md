# Cursor pagination

[English](pagination.md) | [简体中文](zh-CN/pagination.md)

`Pager[T]` provides bounded, context-aware cursor iteration without background goroutines or automatic
prefetch. Applications adapt a contract-specific list operation to the small `PageFetcher[T]` shape.

## Types

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

`MaxPages: 0` selects `DefaultMaxPages` (10,000). Negative values and a nil fetcher return
`PaginationInvalidOptions`.

## Minimal example

This synthetic example shows the pager contract without a live service or credential:

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

For an SDK list operation, the fetcher builds that operation's generated input with the supplied
cursor, calls the public `Client` method, and maps the response's items/`has_more`/`next_cursor` fields
to `CursorPage[T]`. For example, `ListContacts` adapts as follows:

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

The complete [`list-contacts-pager`](../examples/list-contacts-pager/main.go) example reads the account
ID from the process environment and does not print contact records or opaque cursor values. Other
resources can expose different list result shapes; their generated Go types and the Device API
contract remain authoritative.

## Safety invariants

The pager:

- fetches a page only when `Next` needs another item;
- never starts a background goroutine;
- checks `context.Context` before fetching and after the fetcher returns;
- returns fetcher errors unchanged unless the context has already ended;
- requires a non-empty next cursor when `HasMore` is true;
- rejects a cursor already seen, including a repeat of `InitialCursor`;
- stops after `MaxPages`, even if every page is empty and claims more data;
- returns `ok=false, err=nil` after normal completion and on later calls.

One `Pager` is for one sequential consumer. Do not call `Next` concurrently.

## Error handling

Use `errors.As` for `*PaginationError` and inspect `Reason`:

| Reason | Meaning |
| --- | --- |
| `PaginationInvalidOptions` | Nil fetcher, invalid page limit, nil context, or unusable pager |
| `PaginationMissingCursor` | `HasMore` is true but `NextCursor` is empty |
| `PaginationRepeatedCursor` | The server returned a cursor already used |
| `PaginationPageLimit` | Continuing would exceed `MaxPages` |
| `PaginationCanceled` | Context was canceled |
| `PaginationTimedOut` | Context deadline expired |

Cursor values are intentionally absent from `PaginationError`, preventing identifiers or opaque
tokens from entering logs. Canceled/timed-out pagination also supports `errors.Is` with the matching
context sentinel.

## Cancellation responsibility

The fetcher must pass `ctx` into the SDK operation. `Pager` does not create a goroutine to interrupt a
fetcher that ignores context; this avoids leaks but means an incorrectly implemented fetcher can block
its caller.
