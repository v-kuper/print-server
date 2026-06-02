package printcoord

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestCoordinatorLetsWaitingUserPrintPreemptNextFax(t *testing.T) {
	coordinator := New()
	ctx := context.Background()
	var (
		mu    sync.Mutex
		order []string
	)
	appendOrder := func(value string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, value)
	}

	firstFaxRelease := make(chan struct{})
	firstFaxStarted := make(chan struct{})
	firstFaxDone := make(chan struct{})
	go func() {
		ran, err := coordinator.TryRunFax(ctx, func(context.Context) error {
			appendOrder("fax-1-start")
			close(firstFaxStarted)
			<-firstFaxRelease
			appendOrder("fax-1-end")
			return nil
		})
		if err != nil {
			t.Errorf("first fax: %v", err)
		}
		if !ran {
			t.Errorf("expected first fax to run")
		}
		close(firstFaxDone)
	}()
	<-firstFaxStarted

	userDone := make(chan struct{})
	go func() {
		if err := coordinator.RunUserPrint(ctx, func(context.Context) error {
			appendOrder("user")
			return nil
		}); err != nil {
			t.Errorf("user print: %v", err)
		}
		close(userDone)
	}()
	waitUntil(t, coordinator.UserPrintsWaiting)

	ran, err := coordinator.TryRunFax(ctx, func(context.Context) error {
		appendOrder("fax-2")
		return nil
	})
	if err != nil {
		t.Fatalf("second fax: %v", err)
	}
	if ran {
		t.Fatalf("second fax must not run while a user print is waiting")
	}

	close(firstFaxRelease)
	<-firstFaxDone
	<-userDone

	ran, err = coordinator.TryRunFax(ctx, func(context.Context) error {
		appendOrder("fax-3")
		return nil
	})
	if err != nil {
		t.Fatalf("third fax: %v", err)
	}
	if !ran {
		t.Fatalf("expected third fax to run after user print")
	}

	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	want := []string{"fax-1-start", "fax-1-end", "user", "fax-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected order %#v, got %#v", want, got)
	}
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("condition was not met before timeout")
}
