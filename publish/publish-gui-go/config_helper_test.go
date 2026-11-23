package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeShared(t *testing.T, projectPath, serverURL, projectName string) {
	t.Helper()
	dir := filepath.Join(projectPath, ".updator")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"server_url":"` + serverURL + `","project_name":"` + projectName + `"}`
	if err := os.WriteFile(filepath.Join(dir, "shared.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKnownServerURLs(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one")
	second := filepath.Join(root, "two")
	third := filepath.Join(root, "three")
	for _, dir := range []string{first, second, third} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeShared(t, first, "http://server-a:2000", "one")
	writeShared(t, second, "http://SERVER-A:2000", "two") // duplicate, other case
	writeShared(t, third, "http://server-b:2000", "three")

	projects := []ProjectConfig{
		{DisplayName: "one", ProjectPath: first},
		{DisplayName: "two", ProjectPath: second},
		{DisplayName: "broken", ProjectPath: filepath.Join(root, "missing")},
		{DisplayName: "three", ProjectPath: third},
	}
	got := knownServerURLs(projects)
	want := []string{"http://server-a:2000", "http://server-b:2000"}
	if len(got) != len(want) {
		t.Fatalf("knownServerURLs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("knownServerURLs = %v, want %v", got, want)
		}
	}
}

func TestMainExeCandidates(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"b.exe", "A.EXE", "unins000.exe", "setup-x.exe", "readme.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "sub.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := mainExeCandidates(root)
	want := []string{"A.EXE", "b.exe"}
	if len(got) != len(want) {
		t.Fatalf("mainExeCandidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mainExeCandidates = %v, want %v", got, want)
		}
	}
	if candidates := mainExeCandidates(filepath.Join(root, "missing")); candidates != nil {
		t.Fatalf("missing folder should have no candidates: %v", candidates)
	}
}

func TestProjectPathState(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	initialized := filepath.Join(root, "initialized")
	usable := filepath.Join(root, "usable")
	for _, dir := range []string{initialized, usable} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeShared(t, initialized, "http://server:2000", "initialized")
	projects := []ProjectConfig{{DisplayName: "usable", ProjectPath: usable}}

	cases := []struct {
		name string
		path string
		want pathState
	}{
		{"empty", "", pathEmpty},
		{"missing", missing, pathMissing},
		{"initialized", initialized, pathInitialized},
		{"listed", usable, pathListed},
		{"usable", filepath.Join(root, "fresh"), pathMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "usable" {
				fresh := filepath.Join(root, "fresh")
				if err := os.MkdirAll(fresh, 0o755); err != nil {
					t.Fatal(err)
				}
				if got := projectPathState(fresh, projects); got != pathUsable {
					t.Fatalf("state = %v, want usable", got)
				}
				return
			}
			if got := projectPathState(tc.path, projects); got != tc.want {
				t.Fatalf("state = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeForCompare(t *testing.T) {
	root := t.TempDir()
	trailing := root + string(filepath.Separator)
	if got, want := normalizeForCompare(trailing), normalizeForCompare(root); got != want {
		t.Fatalf("trailing separator kept: %q vs %q", got, want)
	}
	if normalizeForCompare("  ") != "" {
		t.Fatal("blank path should normalize to empty")
	}
	if !filepath.IsAbs(normalizeForCompare("relative")) {
		t.Fatalf("relative path was not made absolute")
	}
}
