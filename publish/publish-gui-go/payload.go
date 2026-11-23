package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// payloadManifestName is written by the release packager into the package root,
// next to the GUI executable.
const payloadManifestName = "payload.json"

// payloadManifest mirrors the installer/payload.json written by the release
// packager. The JSON field names must stay in sync with the C# GUI's PkConfig
// model and the manifest files under publish/publish-gui-csharp/installer/.
//
// cli_rid selects the publish-cli/<rid>/ build, client_rid selects the
// client/<rid>/ updater build, and payload_rids lists every runtime id the
// package actually carries. Reading these ids is what lets the Go GUI replace
// the C# one without asking the user to pick a build by hand.
type payloadManifest struct {
	HostRid     string   `json:"host_rid"`
	CliRid      string   `json:"cli_rid"`
	ClientRid   string   `json:"client_rid"`
	PayloadRids []string `json:"payload_rids"`
}

// loadPayloadManifest reads the manifest from baseDir. A missing or malformed
// manifest is deliberately not an error: development layouts have none, and the
// host-based fallback in cliRIDs / updaterRIDsFor still resolves the binaries.
func loadPayloadManifest(baseDir string) payloadManifest {
	path := filepath.Join(baseDir, payloadManifestName)
	raw, err := readConfigFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("读取 %s 失败: %v", path, err)
		}
		return payloadManifest{}
	}
	var m payloadManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		log.Printf("解析 %s 失败: %v", path, err)
		return payloadManifest{}
	}
	return m
}

// mergeRIDs keeps preferred ids in order, drops blank/duplicate entries, then
// appends the host fallback. The fallback matters: a partial or foreign
// manifest must never hide a build that is actually runnable on this machine.
func mergeRIDs(preferred, fallback []string) []string {
	out := make([]string, 0, len(preferred)+len(fallback))
	seen := make(map[string]bool, len(preferred)+len(fallback))
	for _, rid := range append(append([]string{}, preferred...), fallback...) {
		rid = strings.TrimSpace(rid)
		if rid == "" || seen[rid] {
			continue
		}
		seen[rid] = true
		out = append(out, rid)
	}
	return out
}

// cliRIDs is the Go counterpart of PkHelper.PubRids: the packaged cli_rid
// wins, then the manifest's payload_rids, then the host candidates.
func cliRIDs(baseDir string) []string {
	m := loadPayloadManifest(baseDir)
	return mergeRIDs(append([]string{m.CliRid}, m.PayloadRids...), hostRIDs())
}

// updaterRIDsFor is the Go counterpart of PkHelper.UpRids: client_rid wins,
// then payload_rids, then the host candidates.
func updaterRIDsFor(baseDir string) []string {
	m := loadPayloadManifest(baseDir)
	return mergeRIDs(append([]string{m.ClientRid}, m.PayloadRids...), updaterRIDs())
}
