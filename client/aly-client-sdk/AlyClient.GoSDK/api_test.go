package alyclient

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func withTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), testTimeout)
}

func TestCheckUpdateAvailable(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "update_available")
	ctx, cancel := withTimeout(t)
	defer cancel()

	response := NewAPI(fake).CheckUpdate(ctx)
	if !response.IsSuccess {
		t.Fatalf("NG: %s", response.ErrorMsg)
	}
	if !response.Data.HasUpdate || !response.Data.NeedDownloadUpdate {
		t.Fatalf("unexpected data: %+v", response.Data)
	}
	if response.Data.NewVersion != "1.0.1" {
		t.Fatalf("new_version = %q", response.Data.NewVersion)
	}
	if response.Data.ForceUpdate {
		t.Fatal("force_update should be false")
	}
	if len(response.Data.ReleaseNotes) != 2 {
		t.Fatalf("release_notes = %v", response.Data.ReleaseNotes)
	}
}

func TestCheckUpdateAlreadyLatestAndOffline(t *testing.T) {
	fake := fakeClient(t)
	for _, scenario := range []string{"already_latest", "offline"} {
		t.Run(scenario, func(t *testing.T) {
			useScenario(t, scenario)
			ctx, cancel := withTimeout(t)
			defer cancel()
			response := NewAPI(fake).CheckUpdate(ctx)
			if !response.IsSuccess {
				t.Fatalf("NG: %s", response.ErrorMsg)
			}
			if response.Data.HasUpdate {
				t.Fatal("has_update should be false")
			}
		})
	}
}

func TestDownloadProgress(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "update_available")
	ctx, cancel := withTimeout(t)
	defer cancel()

	var progress []DownloadProgressInfo
	response := NewAPI(fake).DownloadUpdate(ctx, DownloadOptions{
		OnProgress: func(info DownloadProgressInfo) {
			progress = append(progress, info)
		},
	})
	if !response.IsSuccess {
		t.Fatalf("NG: %s", response.ErrorMsg)
	}
	if len(progress) != 1 {
		t.Fatalf("progress events = %d", len(progress))
	}
	if progress[0].File != "app.exe" || progress[0].Ratio != 1 ||
		progress[0].Status != DownloadProgressStatusDone {
		t.Fatalf("progress = %+v", progress[0])
	}
	if progress[0].DownloadedBytes != 1000 {
		t.Fatalf("downloaded bytes = %d", progress[0].DownloadedBytes)
	}
}

func TestDownloadPartialFailure(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "download_partial_failure")
	ctx, cancel := withTimeout(t)
	defer cancel()

	response := NewAPI(fake).DownloadUpdate(ctx, DownloadOptions{})
	if response.IsSuccess {
		t.Fatal("partial failure should be NG")
	}
	if !strings.Contains(response.ErrorMsg, "checksum mismatch") {
		t.Fatalf("errorMsg = %q", response.ErrorMsg)
	}
}

func TestDownloadStall(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "download_stall")
	ctx, cancel := withTimeout(t)
	defer cancel()

	started := time.Now()
	response := NewAPI(fake).DownloadUpdate(ctx, DownloadOptions{StallTimeout: time.Second})
	if response.IsSuccess {
		t.Fatal("stall should be NG")
	}
	if !strings.Contains(response.ErrorMsg, "stalled") {
		t.Fatalf("errorMsg = %q", response.ErrorMsg)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("watchdog took %s", elapsed)
	}
}

func TestApplyUpdateReturnsImmediately(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "update_available")
	started := time.Now()
	response := NewAPI(fake).ApplyUpdate(CloseOptions{
		MustCloseProcessNames: "A,B",
		CloseTimeoutSeconds:   45,
	})
	if !response.IsSuccess {
		t.Fatalf("NG: %s", response.ErrorMsg)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("apply waited %s", elapsed)
	}
}

func TestSelfUpdateFlow(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "self_update")
	ctx, cancel := withTimeout(t)
	defer cancel()

	api := NewAPI(fake)
	check := api.CheckSelfUpdate(ctx)
	if !check.IsSuccess || !check.Data.NeedUpdate {
		t.Fatalf("check self update: %+v / %s", check.Data, check.ErrorMsg)
	}
	updated := api.SelfUpdate(ctx)
	if !updated.IsSuccess || !updated.Data.Updated {
		t.Fatalf("self update: %+v / %s", updated.Data, updated.ErrorMsg)
	}
	if updated.Data.OldPath != "aly-client.exe.old" {
		t.Fatalf("old_path = %q", updated.Data.OldPath)
	}
}

func TestRollbackFlow(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "rollback")
	ctx, cancel := withTimeout(t)
	defer cancel()

	api := NewAPI(fake)
	versions := api.ListRollbackVersions(ctx)
	if !versions.IsSuccess || len(versions.Data.Versions) != 2 {
		t.Fatalf("versions: %+v / %s", versions.Data, versions.ErrorMsg)
	}
	rolled := api.Rollback(ctx, "1.0.0", CloseOptions{})
	if !rolled.IsSuccess || rolled.Data.Version != "1.0.0" {
		t.Fatalf("rollback: %+v / %s", rolled.Data, rolled.ErrorMsg)
	}
}

func TestCheckDiffAndClean(t *testing.T) {
	fake := fakeClient(t)
	useScenario(t, "check_diff")
	ctx, cancel := withTimeout(t)
	defer cancel()

	diff := NewAPI(fake).CheckDiff(ctx)
	if !diff.IsSuccess || len(diff.Data.Files) != 1 || diff.Data.Files[0].Path != "app.exe" {
		t.Fatalf("diff: %+v / %s", diff.Data, diff.ErrorMsg)
	}

	useScenario(t, "clean_all")
	cleaned := NewAPI(fake).Clean(ctx, CleanOptions{All: true})
	if !cleaned.IsSuccess || cleaned.Data.Mode != "all" {
		t.Fatalf("clean: %+v / %s", cleaned.Data, cleaned.ErrorMsg)
	}
}

func TestMissingExecutable(t *testing.T) {
	ctx, cancel := withTimeout(t)
	defer cancel()

	missing := filepath.Join(t.TempDir(), "does-not-exist", "aly-client.exe")
	response := NewAPI(missing).CheckUpdate(ctx)
	if response.IsSuccess {
		t.Fatal("missing executable should be NG")
	}
	if response.ErrorMsg == "" {
		t.Fatal("errorMsg should not be empty")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if !strings.Contains(path, "UpdateFolder") {
		t.Fatalf("default path = %q", path)
	}
}
