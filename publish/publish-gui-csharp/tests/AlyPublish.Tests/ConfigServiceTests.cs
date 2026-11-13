using System.IO;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using Xunit;

namespace AlyPublish.Tests;

public sealed class ConfigServiceTests
{
    [Fact]
    public void SaveAndLoad_RoundTripsBookmarks()
    {
        var directory = Path.Combine(Path.GetTempPath(), "AlyPublish.Tests", Path.GetRandomFileName());
        Directory.CreateDirectory(directory);

        try
        {
            var service = new ConfigService(directory);
            service.SaveProjects(
            [
                new ProjectConfig
                {
                    DisplayName = "MyApp",
                    ProjectPath = @"C:\builds\app",
                    MainExePath = "app.exe"
                }
            ]);

            var loaded = service.LoadProjects();

            var project = Assert.Single(loaded);
            Assert.Equal("MyApp", project.DisplayName);
            Assert.Equal(@"C:\builds\app", project.ProjectPath);
            Assert.Equal("app.exe", project.MainExePath);
            Assert.False(File.Exists(Path.Combine(directory, "config.json.tmp")));
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    [Fact]
    public void LoadProjects_MissingFile_ReturnsEmptyList()
    {
        var directory = Path.Combine(Path.GetTempPath(), "AlyPublish.Tests", Path.GetRandomFileName());
        Directory.CreateDirectory(directory);

        try
        {
            var service = new ConfigService(directory);

            Assert.Empty(service.LoadProjects());
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }
}
