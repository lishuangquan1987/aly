using System.IO;
using AlyPublish.Helpers;
using AlyPublish.Models.Local;
using Xunit;

namespace AlyPublish.Tests;

public sealed class ProjectInputHelperTests
{
    [Fact]
    public void IsInitialized_DependsOnSharedJson()
    {
        var directory = CreateTempDirectory();
        try
        {
            Assert.False(ProjectInputHelper.IsInitialized(directory));

            Directory.CreateDirectory(Path.Combine(directory, ".updator"));
            File.WriteAllText(Path.Combine(directory, ".updator", "shared.json"), "{}");

            Assert.True(ProjectInputHelper.IsInitialized(directory));
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    [Fact]
    public void FindMainExeCandidates_SkipsInstallerNoiseAndSorts()
    {
        var directory = CreateTempDirectory();
        try
        {
            foreach (var name in new[] { "tool.exe", "unins000.exe", "app.exe", "setup.exe", "readme.txt" })
            {
                File.WriteAllText(Path.Combine(directory, name), string.Empty);
            }

            var candidates = ProjectInputHelper.FindMainExeCandidates(directory);

            Assert.Equal(["app.exe", "tool.exe"], candidates);
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    [Fact]
    public void SuggestDisplayName_PrefersSharedProjectName()
    {
        var shared = new SharedConfig { ProjectName = "MyApp" };

        Assert.Equal("MyApp", ProjectInputHelper.SuggestDisplayName(@"C:\builds\whatever", shared));
        Assert.Equal("whatever", ProjectInputHelper.SuggestDisplayName(@"C:\builds\whatever\", null));
    }

    [Fact]
    public void ToStoredMainExePath_KeepsInsideProjectRelative()
    {
        var project = @"C:\builds\app";
        var inside = Path.Combine(project, "bin", "app.exe");

        Assert.Equal("bin/app.exe", ProjectInputHelper.ToStoredMainExePath(project, inside));
    }

    [Fact]
    public void TryGetProjectRelativePath_ReturnsSlashSeparatedPath()
    {
        var project = @"C:\builds\app";
        var inside = Path.Combine(project, "Log", "Config");

        Assert.True(ProjectInputHelper.TryGetProjectRelativePath(project, inside, out var relative));
        Assert.Equal("Log/Config", relative);
    }

    [Fact]
    public void TryGetProjectRelativePath_RejectsProjectRootAndOutsidePaths()
    {
        var project = @"C:\builds\app";

        Assert.False(ProjectInputHelper.TryGetProjectRelativePath(project, project, out _));
        Assert.False(ProjectInputHelper.TryGetProjectRelativePath(project, @"C:\builds\other\file.txt", out _));
    }

    private static string CreateTempDirectory()
    {
        var directory = Path.Combine(Path.GetTempPath(), "AlyPublish.Tests", Path.GetRandomFileName());
        Directory.CreateDirectory(directory);
        return directory;
    }
}
