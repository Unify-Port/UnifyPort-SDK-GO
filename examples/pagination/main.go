package main

import (
	"context"
	"fmt"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func main() {
	// 使用合成页面展示 cursor 约束，示例无需 API key，也不会访问外部服务。
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
