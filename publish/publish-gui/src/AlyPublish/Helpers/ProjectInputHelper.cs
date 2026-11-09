using AlyPublish.Models.Local;

namespace AlyPublish.Helpers;

/// <summary>Pure helpers behind the project wizard: detection, suggestions and validation.</summary>
public static class ProjectInputHelper
{
    private static readonly string[] IgnoredExecutablePrefixes = ["unins", "setup"];

    public static bool IsInitialized(string projectPath)
        => !string.IsNullOrWhiteSpace(projectPath)
           && File.Exists(Path.Combine(projectPath, ".updator", "shared.json"));

    /// <summary>Top-level executables, without installer/uninstaller noise.</summary>
    public static IReadOnlyList<string> FindMainExeCandidates(string projectPath)
    {
        if (string.IsNullOrWhiteSpace(projectPath) || !Directory.Exists(projectPath))
        {
            return [];
        }

        try
        {
            return Directory
                .EnumerateFiles(projectPath, "*.exe", SearchOption.TopDirectoryOnly)
                .Select(Path.GetFileName)
                .Where(name => name is not null
                               && !IgnoredExecutablePrefixes.Any(prefix =>
                                   name.StartsWith(prefix, StringComparison.OrdinalIgnoreCase)))
                .Select(name => name!)
                .Order(StringComparer.OrdinalIgnoreCase)
                .ToList();
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            return [];
        }
    }

    public static string SuggestDisplayName(string projectPath, SharedConfig? sharedConfig)
    {
        if (!string.IsNullOrWhiteSpace(sharedConfig?.ProjectName))
        {
            return sharedConfig.ProjectName;
        }

        return Path.GetFileName(Path.TrimEndingDirectorySeparator(projectPath));
    }

    /// <summary>Resolves the stored executable path (relative or absolute) against the project folder.</summary>
    public static string ResolveMainExePath(string projectPath, string mainExePath)
        => Path.IsPathRooted(mainExePath)
            ? mainExePath
            : Path.GetFullPath(Path.Combine(projectPath, mainExePath));

    /// <summary>Stores an executable inside the project as a slash-separated relative path.</summary>
    public static string ToStoredMainExePath(string projectPath, string mainExePath)
    {
        if (string.IsNullOrWhiteSpace(mainExePath))
        {
            return string.Empty;
        }

        if (!Path.IsPathRooted(mainExePath))
        {
            return NormalizeSeparators(mainExePath);
        }

        var relative = Path.GetRelativePath(projectPath, mainExePath);
        return IsOutside(relative) ? mainExePath : NormalizeSeparators(relative);
    }

    private static bool IsOutside(string relativePath)
        => relativePath == ".."
           || relativePath.StartsWith(".." + Path.DirectorySeparatorChar, StringComparison.Ordinal)
           || relativePath.StartsWith(".." + Path.AltDirectorySeparatorChar, StringComparison.Ordinal);

    private static string NormalizeSeparators(string path)
        => path.Replace(Path.DirectorySeparatorChar, '/').Replace(Path.AltDirectorySeparatorChar, '/');
}
