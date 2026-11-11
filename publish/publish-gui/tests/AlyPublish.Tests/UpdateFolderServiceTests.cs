using System.IO;
using AlyPublish.Helpers;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using Newtonsoft.Json;
using Xunit;

namespace AlyPublish.Tests;

/// <summary>
/// The UpdateFolder the GUI builds next to a project: which updater files land there, what client.json
/// says, and how a missing payload is reported. The package root is passed in, so no live GUI is needed.
/// Every case builds its own root directory, because the UpdateFolder is created next to the project folder.
/// </summary>
public sealed class UpdateFolderServiceTests
{
    [Fact]
    public void Create_CopiesThePakageUpdatersAndWritesClientJson()
    {
        var root = CreateTempRoot();
        try
        {
            var pakage = Path.Combine(root, "pakage");
            var project = Path.Combine(root, "proj");
            StagUpdaters(pakage);
            var mainExe = Path.Combine(project, "app.exe");
            WriteFile(mainExe, "exe");

            var warning = UpdateFolderService.Create(project, mainExe, "app", pakage);

            Assert.True(warning is null);

            var updateFolder = Path.Combine(root, "UpdateFolder");
            foreach (var baseName in HostHelper.UpBaseNames)
            {
                var name = Path.Combine(updateFolder, HostHelper.UpFileName(baseName));
                Assert.True(File.Exists(name));
                Assert.Equal("pakage", File.ReadAllText(name));
            }

            var config = JsonConvert.DeserializeObject<UpdateClientConfig>(
                File.ReadAllText(Path.Combine(updateFolder, "client.json")));
            Assert.True(config is not null);
            Assert.Single(config!.MustCloseProcessName, "app");
            Assert.Equal(
                Path.GetFullPath(mainExe),
                Path.GetFullPath(Path.Combine(updateFolder, config.MainExeRelativePath)));
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void Create_ReportsAWarningInsteadOfThrowingWhenThePayloadIsMissing()
    {
        var root = CreateTempRoot();
        try
        {
            var pakage = Path.Combine(root, "pakage");
            var project = Path.Combine(root, "proj");
            var mainExe = Path.Combine(project, "app.exe");
            WriteFile(mainExe, "exe");

            var warning = UpdateFolderService.Create(project, mainExe, "app", pakage);

            Assert.True(warning is not null);
            Assert.True(File.Exists(Path.Combine(root, "UpdateFolder", "client.json")));
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void RefrshUpdaters_ReplacesTheUpdatersButKeepsClientJson()
    {
        var root = CreateTempRoot();
        try
        {
            var pakage = Path.Combine(root, "pakage");
            var project = Path.Combine(root, "proj");
            StagUpdaters(pakage);
            var updateFolder = Path.Combine(root, "UpdateFolder");
            Directory.CreateDirectory(updateFolder);
            var clientJson = Path.Combine(updateFolder, "client.json");
            File.WriteAllText(clientJson, "{ \"hand\": \"edited\" }");

            var upName = HostHelper.UpFileName("aly-client");
            File.WriteAllText(Path.Combine(updateFolder, upName), "stale");

            var warning = UpdateFolderService.RefrshUpdaters(project, pakage);

            Assert.True(warning is null);
            Assert.Equal("pakage", File.ReadAllText(Path.Combine(updateFolder, upName)));
            Assert.Equal("{ \"hand\": \"edited\" }", File.ReadAllText(clientJson));
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void RefrshUpdaters_DoesNothingWithoutAnUpdateFolder()
    {
        var root = CreateTempRoot();
        try
        {
            var pakage = Path.Combine(root, "pakage");
            var project = Path.Combine(root, "proj");
            StagUpdaters(pakage);

            var warning = UpdateFolderService.RefrshUpdaters(project, pakage);

            Assert.True(warning is null);
            Assert.False(Directory.Exists(Path.Combine(root, "UpdateFolder")));
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    /// <summary>Writes both payload updaters of this platform into the fake package root.</summary>
    private static void StagUpdaters(string pakage)
    {
        var rid = HostHelper.UpRidCandidates()[0];
        foreach (var baseName in HostHelper.UpBaseNames)
        {
            var name = Path.Combine(pakage, PkHelper.UpFolderName, rid, HostHelper.UpFileName(baseName));
            WriteFile(name, "pakage");
        }
    }

    private static string CreateTempRoot()
    {
        var root = Path.Combine(Path.GetTempPath(), "AlyPublish.Tests", Path.GetRandomFileName());
        Directory.CreateDirectory(root);
        return root;
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
