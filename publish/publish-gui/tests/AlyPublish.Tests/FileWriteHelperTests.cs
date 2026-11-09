using System.IO;
using AlyPublish.Helpers;
using Xunit;

namespace AlyPublish.Tests;

public sealed class FileWriteHelperTests
{
    [Fact]
    public void WriteAllTextAtomic_CreatesNewFile()
    {
        var directory = CreateTempDirectory();
        try
        {
            var path = Path.Combine(directory, "settings.json");

            FileWriteHelper.WriteAllTextAtomic(path, "first");

            Assert.Equal("first", File.ReadAllText(path));
            Assert.False(File.Exists(path + ".tmp"));
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    [Fact]
    public void WriteAllTextAtomic_OverwritesExistingFile()
    {
        var directory = CreateTempDirectory();
        try
        {
            var path = Path.Combine(directory, "settings.json");
            File.WriteAllText(path, "old");

            FileWriteHelper.WriteAllTextAtomic(path, "new");

            Assert.Equal("new", File.ReadAllText(path));
            Assert.False(File.Exists(path + ".tmp"));
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    private static string CreateTempDirectory()
    {
        var directory = Path.Combine(Path.GetTempPath(), "AlyPublish.Tests", Path.GetRandomFileName());
        Directory.CreateDirectory(directory);
        return directory;
    }
}
