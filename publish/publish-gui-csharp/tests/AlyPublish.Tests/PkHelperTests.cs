using System.IO;
using AlyPublish.Helpers;
using AlyPublish.Models.Local;
using Newtonsoft.Json;
using Xunit;

namespace AlyPublish.Tests;

/// <summary>
/// The packaged payload lookup: which publish build and which updater the GUI picks. Every case builds a
/// fake package tree so the platform never leaks into the asserts.
/// </summary>
public sealed class PkHelperTests
{
    [Fact]
    public void FindPub_PrefersTheManifestRid()
    {
        var app = CreateTempDirectory();
        try
        {
            var name = HostHelper.PubFileName();
            WriteFile(Path.Combine(app, PkHelper.PubFolderName, "linux-amd64", name), "amd64");
            WriteFile(Path.Combine(app, PkHelper.PubFolderName, "linux-arm64", name), "arm64");
            WriteFile(Path.Combine(app, PkHelper.ManifestFileName), Manifest("linux-arm64", "linux-arm64"));

            var found = PkHelper.FindPub(app, string.Empty, PkHelper.PubRids(app));

            Assert.True(found.IsFound);
            Assert.Equal("linux-arm64", found.Rid);
            Assert.False(found.IsFallbackRid);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
        }
    }

    [Fact]
    public void FindPub_ReportsAFallbackWhenOnlyAnotherBuildExists()
    {
        var app = CreateTempDirectory();
        try
        {
            var name = HostHelper.PubFileName();
            WriteFile(Path.Combine(app, PkHelper.PubFolderName, "linux-amd64", name), "amd64");
            WriteFile(Path.Combine(app, PkHelper.ManifestFileName), Manifest("linux-arm64", "linux-arm64"));

            var found = PkHelper.FindPub(app, string.Empty, PkHelper.PubRids(app));

            Assert.True(found.IsFound);
            Assert.Equal("linux-amd64", found.Rid);
            Assert.True(found.IsFallbackRid);
            Assert.Equal("linux-arm64", found.ExpectedRid);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
        }
    }

    [Fact]
    public void FindPub_FallsBackToFlatAndDevelopmentLayouts()
    {
        var app = CreateTempDirectory();
        var project = CreateTempDirectory();
        try
        {
            var name = HostHelper.PubFileName();
            WriteFile(Path.Combine(app, name), "flat");

            var flat = PkHelper.FindPub(app, project, PkHelper.PubRids(app));

            Assert.True(flat.IsFound);
            Assert.Equal(string.Empty, flat.Rid);
            Assert.False(flat.IsFallbackRid);

            File.Delete(Path.Combine(app, name));
            WriteFile(Path.Combine(project, name), "project");

            var inProject = PkHelper.FindPub(app, project, PkHelper.PubRids(app));

            Assert.True(inProject.IsFound);
            Assert.Equal(Path.GetFullPath(Path.Combine(project, name)), inProject.FullName);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
            Directory.Delete(project, recursive: true);
        }
    }

    [Fact]
    public void FindPub_ReturnsNotFoundWithProbedPaths()
    {
        var app = CreateTempDirectory();
        try
        {
            var found = PkHelper.FindPub(app, string.Empty, PkHelper.PubRids(app));

            Assert.False(found.IsFound);
            Assert.Equal(string.Empty, found.FullName);
            Assert.True(found.ProbedPaths.Count > 0);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
        }
    }

    [Fact]
    public void FindUp_FindsBothUpdaterFilenames()
    {
        var app = CreateTempDirectory();
        try
        {
            var rid = HostHelper.UpRidCandidates()[0];
            var upName = HostHelper.UpFileName("aly-client");
            var uiName = HostHelper.UpFileName("aly-client-ui");
            WriteFile(Path.Combine(app, PkHelper.UpFolderName, rid, upName), "up");
            WriteFile(Path.Combine(app, PkHelper.UpFolderName, rid, uiName), "ui");

            var up = PkHelper.FindUp(app, string.Empty, PkHelper.UpRids(app), upName);
            var ui = PkHelper.FindUp(app, string.Empty, PkHelper.UpRids(app), uiName);

            Assert.True(up.IsFound);
            Assert.True(ui.IsFound);
            Assert.Equal(rid, up.Rid);
            Assert.Equal(rid, ui.Rid);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
        }
    }

    [Fact]
    public void ListMissing_NamesEveryPakageFile()
    {
        var app = CreateTempDirectory();
        try
        {
            var missing = PkHelper.ListMissing(app);

            Assert.Equal(PkHelper.PubRidList.Count + (PkHelper.UpRidList.Count * 2), missing.Count);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
        }
    }

    [Fact]
    public void TryLoadManifest_RejectsAnUnparsableFile()
    {
        var app = CreateTempDirectory();
        try
        {
            WriteFile(Path.Combine(app, PkHelper.ManifestFileName), "{ not json");

            var manifest = PkHelper.TryLoadManifest(app);

            Assert.True(manifest is null);
        }
        finally
        {
            Directory.Delete(app, recursive: true);
        }
    }

    /// <summary>
    /// The checked-in manifests are the byts the packages ship: their JSON keys must be exactly the ones
    /// the DTO reads, otherwise the GUI would fall back to platform probing on a real install.
    /// </summary>
    [Fact]
    public void ManifestFiles_KeysAreTheOnesTheGuiReads()
    {
        var root = FindRepoRoot();
        if (root is null)
        {
            return;
        }

        var windows = JsonConvert.DeserializeObject<PkConfig>(File.ReadAllText(Path.Combine(
            root, "publish", "publish-gui-csharp", "installer", "payload.json")));
        Assert.Equal("windows-x64", windows!.HostRid);
        Assert.Equal("windows-amd64", windows.PubRid);
        Assert.Equal("windows-x86", windows.UpRid);
        Assert.Equal("windows-amd64", windows.RidList[0]);

        var linux = JsonConvert.DeserializeObject<PkConfig>(File.ReadAllText(Path.Combine(
            root, "publish", "publish-gui-csharp", "installer", "payload.linux-arm64.json")));
        Assert.Equal("linux-arm64", linux!.HostRid);
        Assert.Equal("linux-arm64", linux.PubRid);
        Assert.Equal("linux-arm64", linux.UpRid);
    }

    /// <summary>The repository root, found by walking up to the packager manifest; null when not present.</summary>
    private static string? FindRepoRoot()
    {
        var directory = Path.GetFullPath(".");
        for (var i = 0; i < 12; i++)
        {
            if (File.Exists(Path.Combine(directory, "publish", "publish-gui-csharp", "installer", "payload.json")))
            {
                return directory;
            }

            var parent = Path.GetDirectoryName(directory);
            if (string.IsNullOrWhiteSpace(parent))
            {
                return null;
            }

            directory = parent;
        }

        return null;
    }

    private static string Manifest(string cliRid, string clientRid) => "{"
        + "\n  \"host_rid\": \"test-host\","
        + "\n  \"cli_rid\": \"" + cliRid + "\","
        + "\n  \"client_rid\": \"" + clientRid + "\","
        + "\n  \"payload_rids\": [\"windows-amd64\", \"linux-amd64\", \"linux-arm64\", \"windows-x86\"]"
        + "\n}";

    private static string CreateTempDirectory()
    {
        var directory = Path.Combine(Path.GetTempPath(), "AlyPublish.Tests", Path.GetRandomFileName());
        Directory.CreateDirectory(directory);
        return directory;
    }

    private static void WriteFile(string path, string content)
    {
        var directory = Path.GetDirectoryName(path);
        if (!string.IsNullOrWhiteSpace(directory) && !Directory.Exists(directory))
        {
            Directory.CreateDirectory(directory);
        }

        File.WriteAllText(path, content);
    }
}
