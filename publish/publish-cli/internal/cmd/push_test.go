package cmd

import (
	"testing"

	"aly/publish-cli/internal/staging"
)

// TestDropLegacyDeleted checks that a staged entry left by an old CLI can
// never turn into a server-side delete now that delete sync is removed.
func TestDropLegacyDeleted(t *testing.T) {
	items := []staging.StagedFile{
		{RelativePath: "keep.dll", Status: "modified"},
		{RelativePath: "gone.dll", Status: "deleted"},
	}
	kept := dropLegacyDeleted(items)
	if len(kept) != 1 || kept[0].RelativePath != "keep.dll" {
		t.Fatalf("legacy deleted 未被剔除: %+v", kept)
	}
}
