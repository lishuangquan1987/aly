namespace AlyPublish.Helpers;

/// <summary>Writes files atomically so a crash never leaves a half-written JSON file.</summary>
public static class FileWriteHelper
{
    public static void WriteAllTextAtomic(string path, string content)
    {
        var temporaryPath = path + ".tmp";
        File.WriteAllText(temporaryPath, content);

        try
        {
            File.Move(temporaryPath, path, overwrite: true);
        }
        catch
        {
            TryDeleteTemporaryFile(temporaryPath);
            throw;
        }
    }

    private static void TryDeleteTemporaryFile(string path)
    {
        try
        {
            File.Delete(path);
        }
        catch (IOException)
        {
            // Best effort cleanup only; the original exception is more useful.
        }
        catch (UnauthorizedAccessException)
        {
            // Best effort cleanup only; the original exception is more useful.
        }
    }
}
