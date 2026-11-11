using System.Globalization;
using AlyPublish.Models.Local;
using Newtonsoft.Json;
using Serilog;

namespace AlyPublish.Helpers;

/// <summary>One probed payload path and the runtime id it belongs to.</summary>
public sealed class PkCandidate
{
    public string FullName { get; set; } = string.Empty;

    /// <summary>The payload runtime id the path belongs to; empty for legagy flat layouts.</summary>
    public string Rid { get; set; } = string.Empty;
}

/// <summary>Where a packaged executable was found, which runtime id it belongs to, and what was probed.</summary>
public sealed class PkLocation
{
    /// <summary>The absolute path of the found executable; empty when nothing was found.</summary>
    public string FullName { get; set; } = string.Empty;

    /// <summary>The payload runtime id the executable belongs to (empty for legagy flat layouts).</summary>
    public string Rid { get; set; } = string.Empty;

    /// <summary>The runtime id the caller asked for first, used to report a fallback to another build.</summary>
    public string ExpectedRid { get; set; } = string.Empty;

    /// <summary>Every path that was probed, for diagnostics and self-testing.</summary>
    public IReadOnlyList<string> ProbedPaths { get; set; } = [];

    public bool IsFound => !string.IsNullOrEmpty(FullName);

    /// <summary>True when a runtime id other than the expected one was used (the payload of another platform).</summary>
    public bool IsFallbackRid =>
        IsFound
        && !string.IsNullOrWhiteSpace(Rid)
        && !string.IsNullOrWhiteSpace(ExpectedRid)
        && !Rid.Equals(ExpectedRid, StringComparison.OrdinalIgnoreCase);

    public static PkLocation NotFound(IReadOnlyList<string> probedPaths)
        => new PkLocation { ProbedPaths = probedPaths };

    public static PkLocation Found(
        string fullName,
        string rid,
        string expectedRid,
        IReadOnlyList<string> probedPaths)
        => new PkLocation
        {
            FullName = fullName,
            Rid = rid,
            ExpectedRid = expectedRid,
            ProbedPaths = probedPaths
        };
}

/// <summary>
/// Locates the packaged payload inside the AlyPublish package folder: <c>payload.json</c>,
/// <c>publish-cli/{rid}/aly-publish(.exe)</c> and <c>client/{rid}/aly-client(-ui)(.exe)</c>. Every method
/// takes explicit root directories, so the selection is unit-testable without a live GUI.
/// </summary>
public static class PkHelper
{
    public const string ManifestFileName = "payload.json";
    public const string PubFolderName = "publish-cli";
    public const string UpFolderName = "client";

    /// <summary>The updater executable that lives next to the updater itself inside the payload.</summary>
    public const string UpBaseName = "aly-client";

    /// <summary>Runtime ids the released packages carry (used by the package self-check).</summary>
    public static readonly IReadOnlyList<string> PubRidList = ["windows-amd64", "linux-amd64", "linux-arm64"];
    public static readonly IReadOnlyList<string> UpRidList = ["windows-x86", "linux-amd64", "linux-arm64"];

    /// <summary>Reads <c>payload.json</c> from the package root; <c>null</c> when absent or unparsable.</summary>
    public static PkConfig? TryLoadManifest(string baseDirectory)
    {
        var path = Path.Combine(baseDirectory, ManifestFileName);
        if (!File.Exists(path))
        {
            return null;
        }

        try
        {
            return JsonConvert.DeserializeObject<PkConfig>(File.ReadAllText(path));
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or JsonException)
        {
            Log.Warning(ex, "读取 payload.json 失败: {Path}", path);
            return null;
        }
    }

    /// <summary>
    /// The publish tool runtime ids to probe, in order: the one named by <c>payload.json</c> first
    /// (authoritative for a shipped package), then every rid that package carries, then the platform
    /// candidates (development and legagy layouts).
    /// </summary>
    public static IReadOnlyList<string> PubRids(string baseDirectory)
    {
        var manifest = TryLoadManifest(baseDirectory);
        var fromManifest = manifest is null
            ? new List<string>()
            : Prepend(manifest.PubRid, manifest.RidList);
        return Combine(fromManifest, HostHelper.PubRidCandidates());
    }

    /// <summary>The updater runtime ids to probe, <c>payload.json</c> first, then the platform candidates.</summary>
    public static IReadOnlyList<string> UpRids(string baseDirectory)
    {
        var manifest = TryLoadManifest(baseDirectory);
        var fromManifest = manifest is null
            ? new List<string>()
            : Prepend(manifest.UpRid, manifest.RidList);
        return Combine(fromManifest, HostHelper.UpRidCandidates());
    }

    /// <summary>
    /// Every path probed for the packaged publish tool, in probe order: the payload directory, a legagy flat
    /// copy next to the GUI, the project directory, then the development tree.
    /// </summary>
    public static IReadOnlyList<PkCandidate> PubProbes(
        string baseDirectory,
        string projectDirectory,
        IReadOnlyList<string> ridCandidates)
    {
        var fileName = HostHelper.PubFileName();
        var candidates = new List<PkCandidate>();
        foreach (var rid in ridCandidates)
        {
            candidates.Add(new PkCandidate
            {
                FullName = Path.Combine(baseDirectory, PubFolderName, rid, fileName),
                Rid = rid
            });
        }

        candidates.Add(new PkCandidate { FullName = Path.Combine(baseDirectory, fileName) });
        if (!string.IsNullOrWhiteSpace(projectDirectory))
        {
            candidates.Add(new PkCandidate { FullName = Path.Combine(projectDirectory, fileName) });
        }

        candidates.Add(new PkCandidate
        {
            FullName = Path.GetFullPath(Path.Combine(
                baseDirectory, "..", "..", "..", "..", "..", "..", PubFolderName, fileName))
        });

        return candidates;
    }

    /// <summary>Every path probed for one packaged updater executable, in probe order.</summary>
    public static IReadOnlyList<PkCandidate> UpProbes(
        string baseDirectory,
        string projectDirectory,
        IReadOnlyList<string> ridCandidates,
        string fileName)
    {
        var candidates = new List<PkCandidate>();
        foreach (var rid in ridCandidates)
        {
            candidates.Add(new PkCandidate
            {
                FullName = Path.Combine(baseDirectory, UpFolderName, rid, fileName),
                Rid = rid
            });
        }

        candidates.Add(new PkCandidate { FullName = Path.Combine(baseDirectory, fileName) });
        if (!string.IsNullOrWhiteSpace(projectDirectory))
        {
            candidates.Add(new PkCandidate { FullName = Path.Combine(projectDirectory, fileName) });
        }

        candidates.Add(new PkCandidate
        {
            FullName = Path.GetFullPath(Path.Combine(
                baseDirectory,
                "..",
                "..",
                "..",
                "..",
                "..",
                "..",
                "..",
                UpFolderName,
                UpBaseName,
                fileName))
        });

        return candidates;
    }

    /// <summary>Finds the packaged publish tool; see <see cref="PubProbes"/> for the probe order.</summary>
    public static PkLocation FindPub(
        string baseDirectory,
        string projectDirectory,
        IReadOnlyList<string> ridCandidates)
    {
        var candidates = PubProbes(baseDirectory, projectDirectory, ridCandidates);
        var probed = candidates.Select(candidate => candidate.FullName).ToList();
        var expectedRid = ridCandidates.Count == 0 ? string.Empty : ridCandidates[0];

        foreach (var candidate in candidates)
        {
            if (File.Exists(candidate.FullName))
            {
                return PkLocation.Found(candidate.FullName, candidate.Rid, expectedRid, probed);
            }
        }

        return PkLocation.NotFound(probed);
    }

    /// <summary>Finds one packaged updater executable (<c>aly-client</c> or <c>aly-client-ui</c>).</summary>
    public static PkLocation FindUp(
        string baseDirectory,
        string projectDirectory,
        IReadOnlyList<string> ridCandidates,
        string fileName)
    {
        var candidates = UpProbes(baseDirectory, projectDirectory, ridCandidates, fileName);
        var probed = candidates.Select(candidate => candidate.FullName).ToList();
        var expectedRid = ridCandidates.Count == 0 ? string.Empty : ridCandidates[0];

        foreach (var candidate in candidates)
        {
            if (File.Exists(candidate.FullName))
            {
                return PkLocation.Found(candidate.FullName, candidate.Rid, expectedRid, probed);
            }
        }

        return PkLocation.NotFound(probed);
    }

    /// <summary>
    /// The payload files a complete package must carry: one publish build per publish rid and both updater
    /// executables per updater rid. Shared by the packager self-check and the GUI diagnostics, so the names
    /// live in one place.
    /// </summary>
    public static IReadOnlyList<string> ListMissing(string appDirectory)
    {
        var missing = new List<string>();
        foreach (var rid in PubRidList)
        {
            var name = Path.Combine(appDirectory, PubFolderName, rid, HostHelper.PubFileName(IsWindowsRid(rid)));
            if (!File.Exists(name))
            {
                missing.Add(name);
            }
        }

        foreach (var rid in UpRidList)
        {
            foreach (var baseName in HostHelper.UpBaseNames)
            {
                var name = Path.Combine(
                    appDirectory,
                    UpFolderName,
                    rid,
                    HostHelper.UpFileName(baseName, IsWindowsRid(rid)));
                if (!File.Exists(name))
                {
                    missing.Add(name);
                }
            }
        }

        return missing;
    }

    private static bool IsWindowsRid(string rid)
        => rid.StartsWith("windows", StringComparison.OrdinalIgnoreCase);

    private static List<string> Prepend(string? preffered, IReadOnlyList<string> rest)
    {
        var result = new List<string>();
        if (!string.IsNullOrWhiteSpace(preffered))
        {
            result.Add(preffered.Trim());
        }

        return Combine(result, rest);
    }

    private static List<string> Combine(IReadOnlyList<string> first, IReadOnlyList<string> second)
    {
        var result = new List<string>();
        foreach (var item in first)
        {
            if (!result.Contains(item))
            {
                result.Add(item);
            }
        }

        foreach (var item in second)
        {
            if (!result.Contains(item))
            {
                result.Add(item);
            }
        }

        return result;
    }
}

