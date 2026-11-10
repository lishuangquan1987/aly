package alyclient

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestManagedForceUpdate(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "force_update")

	client := NewUpdateClient(fake)
	var mu sync.Mutex
	var statuses []Status
	client.OnStatusChange = func(status Status, _ string) {
		mu.Lock()
		statuses = append(statuses, status)
		mu.Unlock()
	}

	client.Start()
	waitFor(t, func() bool { return client.Status() == StatusApplyUpdate }, 5*time.Second)
	client.Stop()

	mu.Lock()
	got := append([]Status(nil), statuses...)
	mu.Unlock()

	if len(got) == 0 || got[0] != StatusDiscoveredUpdate {
		t.Fatalf("statuses = %v", got)
	}
	if !containsStatus(got, StatusDownloadingUpdate) || !containsStatus(got, StatusDownloadedUpdate) {
		t.Fatalf("statuses = %v", got)
	}
	if got[len(got)-1] != StatusApplyUpdate {
		t.Fatalf("last status = %v", got)
	}
	if errMsg := client.LastError(); errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if client.IsRunning() {
		t.Fatal("client should be stopped")
	}
}

func TestManagedNonForceWaitsForConfirmations(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "update_available")

	client := NewUpdateClient(fake)
	var mu sync.Mutex
	var downloads, applies []string
	client.ConfirmDownload = func(_ context.Context, version string) {
		mu.Lock()
		downloads = append(downloads, version)
		mu.Unlock()
	}
	client.ConfirmApply = func(_ context.Context, version string) {
		mu.Lock()
		applies = append(applies, version)
		mu.Unlock()
	}

	client.Start()
	waitFor(t, func() bool { return client.Status() == StatusApplyUpdate }, 5*time.Second)
	client.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(downloads) != 1 || downloads[0] != "1.0.1" {
		t.Fatalf("download confirms = %v", downloads)
	}
	if len(applies) != 1 || applies[0] != "1.0.1" {
		t.Fatalf("apply confirms = %v", applies)
	}
}

func TestManagedOfflineStaysIdle(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "offline")

	client := NewUpdateClient(fake)
	client.ConfirmDownload = func(context.Context, string) {
		t.Error("confirmDownload must not be called when offline")
	}

	client.Start()
	time.Sleep(300 * time.Millisecond)
	if status := client.Status(); status != StatusNone {
		t.Fatalf("status = %q", status)
	}
	client.Stop()
}

func TestStopInterruptsPendingConfirmation(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "update_available")

	client := NewUpdateClient(fake)
	client.ConfirmDownload = func(ctx context.Context, _ string) {
		<-ctx.Done()
	}

	client.Start()
	waitFor(t, func() bool { return client.Status() == StatusDiscoveredUpdate }, 5*time.Second)

	started := time.Now()
	client.Stop()
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("Stop waited %s on a pending confirmation", elapsed)
	}
	if client.IsRunning() {
		t.Fatal("client should be stopped")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "offline")

	client := NewUpdateClient(fake)
	client.Start()
	time.Sleep(100 * time.Millisecond)
	client.Stop()
	client.Stop()
	if client.IsRunning() {
		t.Fatal("client should be stopped")
	}
}

func containsStatus(statuses []Status, want Status) bool {
	for _, status := range statuses {
		if status == want {
			return true
		}
	}
	return false
}
