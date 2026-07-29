package unifyport

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestPagerFetchesLazilyAndHandlesEmptyPages(t *testing.T) {
	t.Parallel()
	pages := map[string]CursorPage[int]{
		"initial": {Items: []int{1, 2}, HasMore: true, NextCursor: "second"},
		"second":  {Items: nil, HasMore: true, NextCursor: "third"},
		"third":   {Items: []int{3}, HasMore: false},
	}
	fetches := 0
	pager, err := NewPager(func(_ context.Context, cursor string) (CursorPage[int], error) {
		fetches++
		return pages[cursor], nil
	}, PagerOptions{InitialCursor: "initial"})
	if err != nil {
		t.Fatalf("create pager: %v", err)
	}
	if fetches != 0 {
		t.Fatal("pager prefetched a page")
	}

	var items []int
	for {
		item, ok, nextErr := pager.Next(context.Background())
		if nextErr != nil {
			t.Fatalf("next item: %v", nextErr)
		}
		if !ok {
			break
		}
		items = append(items, item)
		if len(items) == 1 && fetches != 1 {
			t.Fatalf("pager fetched ahead: %d", fetches)
		}
	}
	if !reflect.DeepEqual(items, []int{1, 2, 3}) || fetches != 3 {
		t.Fatalf("items=%v fetches=%d", items, fetches)
	}
}

func TestPagerDetectsBrokenCursorProtocol(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		page       CursorPage[int]
		initial    string
		wantReason PaginationErrorReason
	}{
		{name: "missing", page: CursorPage[int]{HasMore: true}, wantReason: PaginationMissingCursor},
		{name: "repeated initial", page: CursorPage[int]{HasMore: true, NextCursor: "same"}, initial: "same", wantReason: PaginationRepeatedCursor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pager, err := NewPager(func(context.Context, string) (CursorPage[int], error) {
				return test.page, nil
			}, PagerOptions{InitialCursor: test.initial})
			if err != nil {
				t.Fatalf("create pager: %v", err)
			}
			_, _, err = pager.Next(context.Background())
			var paginationError *PaginationError
			if !errors.As(err, &paginationError) || paginationError.Reason != test.wantReason {
				t.Fatalf("expected %s, got %v", test.wantReason, err)
			}
		})
	}
}

func TestPagerYieldsCurrentPageBeforeReportingBrokenContinuation(t *testing.T) {
	t.Parallel()
	pager, err := NewPager(func(context.Context, string) (CursorPage[int], error) {
		return CursorPage[int]{Items: []int{1, 2}, HasMore: true}, nil
	}, PagerOptions{})
	if err != nil {
		t.Fatalf("create pager: %v", err)
	}
	for _, expected := range []int{1, 2} {
		item, ok, nextErr := pager.Next(context.Background())
		if nextErr != nil || !ok || item != expected {
			t.Fatalf("item=%d ok=%t err=%v", item, ok, nextErr)
		}
	}
	_, _, err = pager.Next(context.Background())
	var paginationError *PaginationError
	if !errors.As(err, &paginationError) || paginationError.Reason != PaginationMissingCursor {
		t.Fatalf("expected missing cursor after current page, got %v", err)
	}
}

func TestPagerEnforcesPageLimitAfterYieldingCurrentPage(t *testing.T) {
	t.Parallel()
	pager, err := NewPager(func(_ context.Context, cursor string) (CursorPage[string], error) {
		return CursorPage[string]{Items: []string{cursor}, HasMore: true, NextCursor: cursor + "x"}, nil
	}, PagerOptions{InitialCursor: "a", MaxPages: 1})
	if err != nil {
		t.Fatalf("create pager: %v", err)
	}
	item, ok, err := pager.Next(context.Background())
	if err != nil || !ok || item != "a" {
		t.Fatalf("unexpected first item: %q %t %v", item, ok, err)
	}
	_, _, err = pager.Next(context.Background())
	var paginationError *PaginationError
	if !errors.As(err, &paginationError) || paginationError.Reason != PaginationPageLimit || paginationError.MaxPages != 1 {
		t.Fatalf("unexpected limit error: %v", err)
	}
}

func TestPagerHonorsContextBetweenBufferedItems(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	pager, err := NewPager(func(context.Context, string) (CursorPage[int], error) {
		return CursorPage[int]{Items: []int{1, 2}}, nil
	}, PagerOptions{})
	if err != nil {
		t.Fatalf("create pager: %v", err)
	}
	if _, ok, nextErr := pager.Next(ctx); nextErr != nil || !ok {
		t.Fatalf("unexpected first item: ok=%t err=%v", ok, nextErr)
	}
	cancel()
	_, _, err = pager.Next(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestPagerPreservesFetcherErrors(t *testing.T) {
	t.Parallel()
	want := fmt.Errorf("sanitized fetch error")
	pager, err := NewPager(func(context.Context, string) (CursorPage[int], error) {
		return CursorPage[int]{}, want
	}, PagerOptions{})
	if err != nil {
		t.Fatalf("create pager: %v", err)
	}
	_, _, err = pager.Next(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("expected fetcher error, got %v", err)
	}
}

func TestNewPagerRejectsInvalidOptions(t *testing.T) {
	t.Parallel()
	if _, err := NewPager[int](nil, PagerOptions{}); err == nil {
		t.Fatal("expected nil fetcher error")
	}
	if _, err := NewPager(func(context.Context, string) (CursorPage[int], error) {
		return CursorPage[int]{}, nil
	}, PagerOptions{MaxPages: -1}); err == nil {
		t.Fatal("expected invalid max pages error")
	}
}

func contextCanceled() error {
	return context.Canceled
}
