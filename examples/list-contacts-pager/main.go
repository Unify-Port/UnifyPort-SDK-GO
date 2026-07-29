package main

import (
	"context"
	"fmt"
	"log"
	"os"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func main() {
	client, err := unifyport.NewClient(unifyport.Config{
		BaseURL: requiredEnv("UNIFYPORT_DEVICE_API_BASE_URL"),
		APIKeyProvider: unifyport.StaticAPIKey(
			requiredEnv("UNIFYPORT_DEVICE_API_KEY"),
		),
	})
	if err != nil {
		log.Fatal(err)
	}
	accountID := requiredEnv("UNIFYPORT_ACCOUNT_ID")

	// Fetcher 只映射公开 list response，不缓存或输出 contact 与 cursor 内容。
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

	pager, err := unifyport.NewPager(fetcher, unifyport.PagerOptions{})
	if err != nil {
		log.Fatal(err)
	}

	count := 0
	for {
		_, ok, err := pager.Next(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		if !ok {
			break
		}
		count++
	}
	// 只输出总数，避免把 contact 或 opaque cursor 写入日志。
	fmt.Printf("contacts=%d\n", count)
}

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		log.Fatalf("missing required environment variable: %s", name)
	}
	return value
}
