package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func writeManifest(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, payloadManifestName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPayloadManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
  "host_rid": "linux-x64",
  "cli_rid": "linux-amd64",
  "client_rid": "linux-arm64",
  "payload_rids": ["windows-amd64", "linux-amd64", "linux-arm64", "windows-x86"]
}`)

	m := loadPayloadManifest(dir)
	if m.HostRid != "linux-x64" || m.CliRid != "linux-amd64" || m.ClientRid != "linux-arm64" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if !reflect.DeepEqual(m.PayloadRids, []string{"windows-amd64", "linux-amd64", "linux-arm64", "windows-x86"}) {
		t.Fatalf("unexpected payload_rids: %v", m.PayloadRids)
	}
}

func TestCLIRIDsPrefersManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{"cli_rid":"linux-arm64","payload_rids":["linux-arm64","windows-amd64"]}`)

	got := cliRIDs(dir)
	if len(got) < 2 || got[0] != "linux-arm64" || got[1] != "windows-amd64" {
		t.Fatalf("manifest rids must lead the probe order: %v", got)
	}
	for _, rid := range hostRIDs() {
		if !slices.Contains(got, rid) {
			t.Fatalf("host fallback %q missing from %v", rid, got)
		}
	}
}

func TestUpdaterRIDsPrefersClientRid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{"client_rid":"windows-x86","payload_rids":["windows-x86","linux-amd64"]}`)

	got := updaterRIDsFor(dir)
	if len(got) == 0 || got[0] != "windows-x86" {
		t.Fatalf("client_rid must lead the probe order: %v", got)
	}
	for _, rid := range updaterRIDs() {
		if !slices.Contains(got, rid) {
			t.Fatalf("host fallback %q missing from %v", rid, got)
		}
	}
}

func TestRIDsFallbackWithoutUsableManifest(t *testing.T) {
	dir := t.TempDir()
	if got := cliRIDs(dir); !reflect.DeepEqual(got, hostRIDs()) {
		t.Fatalf("missing manifest should fall back to host rids: got %v want %v", got, hostRIDs())
	}
	if got := updaterRIDsFor(dir); !reflect.DeepEqual(got, updaterRIDs()) {
		t.Fatalf("missing manifest should fall back to host updater rids: got %v want %v", got, updaterRIDs())
	}

	writeManifest(t, dir, `{not json`)
	if got := cliRIDs(dir); !reflect.DeepEqual(got, hostRIDs()) {
		t.Fatalf("malformed manifest should fall back to host rids: %v", got)
	}

	writeManifest(t, dir, `{"cli_rid":"  ","client_rid":"","payload_rids":["",""]}`)
	if got := cliRIDs(dir); !reflect.DeepEqual(got, hostRIDs()) {
		t.Fatalf("blank rids must not enter the probe order: %v", got)
	}
}

func TestMergeRIDsDeduplicates(t *testing.T) {
	got := mergeRIDs(
		[]string{" ", "linux-arm64", "linux-arm64", ""},
		[]string{"linux-amd64", "linux-arm64"},
	)
	want := []string{"linux-arm64", "linux-amd64"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// TestPayloadDrivesBinaryLookup proves the manifest actually decides which
// packaged build is used, not just that the rid list looks right. The probe
// rids are intentionally absent from the host candidates, so without the
// manifest both lookups would come back empty.
func TestPayloadDrivesBinaryLookup(t *testing.T) {
	t.Setenv("ALY_PUBLISH", "")
	dir := t.TempDir()
	const cliRid = "probe-cli-rid"
	const clientRid = "probe-client-rid"
	writeManifest(t, dir, `{"cli_rid":"`+cliRid+`","client_rid":"`+clientRid+`","payload_rids":["`+cliRid+`","`+clientRid+`"]}`)

	cliName := executableName(cliRid)
	cliPath := filepath.Join(dir, "publish-cli", cliRid, cliName)
	if err := os.MkdirAll(filepath.Dir(cliPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliPath, []byte("cli"), 0o755); err != nil {
		t.Fatal(err)
	}

	upName := updaterNames()[0]
	upPath := filepath.Join(dir, "client", clientRid, upName)
	if err := os.MkdirAll(filepath.Dir(upPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(upPath, []byte("updater"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := findCLIIn(dir); got != cliPath {
		t.Fatalf("findCLIIn did not honor cli_rid: got %q want %q", got, cliPath)
	}
	if got := findUpdaterFileIn(dir, upName); got != upPath {
		t.Fatalf("findUpdaterFileIn did not honor client_rid: got %q want %q", got, upPath)
	}
}

// TestInstallerManifestsUseExpectedFields guards against field-name drift
// between the Go GUI and the packager that writes payload.json into every
// release package (both the C# and the Go flavor share those files).
func TestInstallerManifestsUseExpectedFields(t *testing.T) {
	base := filepath.Join("..", "publish-gui-csharp", "installer")
	for _, name := range []string{"payload.json", "payload.linux-x64.json", "payload.linux-arm64.json"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(base, name))
			if os.IsNotExist(err) {
				t.Skipf("installer manifest %s not present in this checkout", name)
			}
			if err != nil {
				t.Fatal(err)
			}
			var m payloadManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if m.HostRid == "" || m.CliRid == "" || m.ClientRid == "" || len(m.PayloadRids) == 0 {
				t.Fatalf("%s: manifest fields not understood by the Go GUI: %+v", name, m)
			}
		})
	}
}
