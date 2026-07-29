package unifyport

import "context"

const DefaultMaxPages = 10_000

// CursorPage 是 Device API cursor response 的语言中立分页视图。
type CursorPage[T any] struct {
	Items      []T
	HasMore    bool
	NextCursor string
}

// PageFetcher 必须遵守 ctx；Pager 不启动后台 goroutine，因此不会为忽略 ctx 的实现制造 goroutine 泄漏。
type PageFetcher[T any] func(ctx context.Context, cursor string) (CursorPage[T], error)

// PagerOptions 控制初始 cursor 和有限页数上限。
type PagerOptions struct {
	InitialCursor string
	MaxPages      int
}

// Pager 按需获取页面，不会自动预取下一页。
type Pager[T any] struct {
	fetcher       PageFetcher[T]
	cursor        string
	seen          map[string]struct{}
	maxPages      int
	pagesFetched  int
	items         []T
	nextItem      int
	doneAfterPage bool
	pendingCursor string
	hasPending    bool
	done          bool
}

// NewPager 创建一个单消费者 Pager；同一个 Pager 不应并发调用 Next。
func NewPager[T any](fetcher PageFetcher[T], options PagerOptions) (*Pager[T], error) {
	if fetcher == nil {
		return nil, &PaginationError{Reason: PaginationInvalidOptions}
	}
	maxPages := options.MaxPages
	if maxPages == 0 {
		maxPages = DefaultMaxPages
	}
	if maxPages < 0 {
		return nil, &PaginationError{Reason: PaginationInvalidOptions}
	}
	seen := make(map[string]struct{})
	if options.InitialCursor != "" {
		seen[options.InitialCursor] = struct{}{}
	}
	return &Pager[T]{
		fetcher:  fetcher,
		cursor:   options.InitialCursor,
		seen:     seen,
		maxPages: maxPages,
	}, nil
}

// Next 返回下一个 item；ok=false 且 err=nil 表示分页正常结束。
func (pager *Pager[T]) Next(ctx context.Context) (item T, ok bool, err error) {
	if ctx == nil {
		return item, false, &PaginationError{Reason: PaginationInvalidOptions}
	}
	if pager == nil || pager.fetcher == nil {
		return item, false, &PaginationError{Reason: PaginationInvalidOptions}
	}
	if pager.done {
		return item, false, nil
	}

	for {
		if err := paginationContextError(ctx); err != nil {
			return item, false, err
		}
		if pager.nextItem < len(pager.items) {
			item = pager.items[pager.nextItem]
			pager.nextItem++
			return item, true, nil
		}
		if pager.doneAfterPage {
			pager.done = true
			return item, false, nil
		}
		if pager.hasPending {
			if pager.pendingCursor == "" {
				return item, false, &PaginationError{Reason: PaginationMissingCursor}
			}
			if _, exists := pager.seen[pager.pendingCursor]; exists {
				return item, false, &PaginationError{Reason: PaginationRepeatedCursor}
			}
			pager.seen[pager.pendingCursor] = struct{}{}
			pager.cursor = pager.pendingCursor
			pager.pendingCursor = ""
			pager.hasPending = false
		}
		if pager.pagesFetched >= pager.maxPages {
			return item, false, &PaginationError{Reason: PaginationPageLimit, MaxPages: pager.maxPages}
		}

		page, fetchErr := pager.fetcher(ctx, pager.cursor)
		if contextErr := paginationContextError(ctx); contextErr != nil {
			return item, false, contextErr
		}
		if fetchErr != nil {
			return item, false, fetchErr
		}
		pager.pagesFetched++
		pager.items = page.Items
		pager.nextItem = 0
		pager.doneAfterPage = !page.HasMore
		if page.HasMore {
			// 当前页 items 必须先交付；continuation cursor 在下一次取页前校验。
			pager.pendingCursor = page.NextCursor
			pager.hasPending = true
		}
	}
}

func paginationContextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return &PaginationError{Reason: PaginationCanceled}
	case context.DeadlineExceeded:
		return &PaginationError{Reason: PaginationTimedOut}
	default:
		return nil
	}
}
