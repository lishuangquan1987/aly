using System.Globalization;

namespace AlyPublish.Helpers;

/// <summary>
/// Host facts the rest of the GUI depends on: the file name conventions of this platform and the runtime
/// ids a packaged payload can provide. Every method takes the platform as an argument so it stays
/// unit-testable; the no-argument overloads read the real host.
/// </summary>
public static class HostHelper
{
    /// <summary>True on Windows hosts; packaged executables carry the <c>.exe</c> suffix there.</summary>
    public static bool IsWindows() => OperatingSystem.IsWindows();

    /// <summary>The publish tool file name on the given platform.</summary>
    public static string PubFileName(bool isWindows) => isWindows ? "aly-publish.exe" : "aly-publish";

    /// <summary>The publish tool file name on this host.</summary>
    public static string PubFileName() => PubFileName(IsWindows());

    /// <summary>The file name of an updater executable on the given platform.</summary>
    public static string UpFileName(string baseName, bool isWindows)
        => isWindows ? baseName + ".exe" : baseName;

    /// <summary>The file name of an updater executable on this host.</summary>
    public static string UpFileName(string baseName) => UpFileName(baseName, IsWindows());

    /// <summary>The updater executables that live next to each other inside <c>UpdateFolder</c>.</summary>
    public static IReadOnlyList<string> UpBaseNames => ["aly-client", "aly-client-ui"];

    /// <summary>
    /// The publish tool runtime ids this platform can run, first-listed one wins. Windows ships one publish
    /// build; Linux ships amd64 and arm64, and amd64 is tried first unless <c>payload.json</c> says otherwise.
    /// </summary>
    public static IReadOnlyList<string> PubRidCandidates(bool isWindows)
        => isWindows ? ["windows-amd64"] : ["linux-amd64", "linux-arm64"];

    /// <summary>The publish tool runtime ids this host can run, first-listed one wins.</summary>
    public static IReadOnlyList<string> PubRidCandidates() => PubRidCandidates(IsWindows());

    /// <summary>
    /// The updater runtime ids this platform can run, first-listed one wins. The Windows updater is the
    /// 32-bit XP-compatible build (<c>windows-x86</c>); Linux matches the publish tool platform ids.
    /// </summary>
    public static IReadOnlyList<string> UpRidCandidates(bool isWindows)
        => isWindows ? ["windows-x86"] : ["linux-amd64", "linux-arm64"];

    /// <summary>The updater runtime ids this host can run, first-listed one wins.</summary>
    public static IReadOnlyList<string> UpRidCandidates() => UpRidCandidates(IsWindows());
}
