using AlyPublish.Helpers;
using Xunit;

namespace AlyPublish.Tests;

/// <summary>The platform facts the package selection is built on.</summary>
public sealed class HostHelperTests
{
    [Fact]
    public void PubFileName_AddsTheExeSuffixOnlyOnWindows()
    {
        Assert.Equal("aly-publish.exe", HostHelper.PubFileName(true));
        Assert.Equal("aly-publish", HostHelper.PubFileName(false));
    }

    [Fact]
    public void UpFileName_AddsTheExeSuffixOnlyOnWindows()
    {
        Assert.Equal("aly-client.exe", HostHelper.UpFileName("aly-client", true));
        Assert.Equal("aly-client", HostHelper.UpFileName("aly-client", false));
        Assert.Equal("aly-client-ui.exe", HostHelper.UpFileName("aly-client-ui", true));
        Assert.Equal("aly-client-ui", HostHelper.UpFileName("aly-client-ui", false));
    }

    [Fact]
    public void RidCandidates_NameOneWindowsBuildAndBothLinuxBilds()
    {
        Assert.Equal("windows-amd64", HostHelper.PubRidCandidates(true)[0]);
        Assert.Equal("windows-x86", HostHelper.UpRidCandidates(true)[0]);

        Assert.Equal("linux-amd64", HostHelper.PubRidCandidates(false)[0]);
        Assert.Equal("linux-arm64", HostHelper.PubRidCandidates(false)[1]);
        Assert.Equal("linux-amd64", HostHelper.UpRidCandidates(false)[0]);
        Assert.Equal("linux-arm64", HostHelper.UpRidCandidates(false)[1]);
    }

    [Fact]
    public void UpBaseNames_CoverTheUpdaterAndItsFrontEnd()
    {
        Assert.Equal(2, HostHelper.UpBaseNames.Count);
        Assert.Equal("aly-client", HostHelper.UpBaseNames[0]);
        Assert.Equal("aly-client-ui", HostHelper.UpBaseNames[1]);
    }
}
