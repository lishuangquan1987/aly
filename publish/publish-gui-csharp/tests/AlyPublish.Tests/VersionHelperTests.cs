using AlyPublish.Helpers;
using Xunit;

namespace AlyPublish.Tests;

public sealed class VersionHelperTests
{
    [Theory]
    [InlineData("V1.0.1", "V1.0.2")]
    [InlineData("1.9", "1.10")]
    [InlineData("V2", "V3")]
    [InlineData("v1.2.3", "v1.2.4")]
    [InlineData(" 1.0 ", "1.1")]
    public void SuggestNext_IncrementsLastSegment(string current, string expected)
    {
        Assert.Equal(expected, VersionHelper.SuggestNext(current));
    }

    [Theory]
    [InlineData("")]
    [InlineData("   ")]
    [InlineData("release")]
    [InlineData("1.a")]
    [InlineData("V1.0.1-beta")]
    public void SuggestNext_ReturnsNullForUnsupportedValues(string current)
    {
        Assert.Null(VersionHelper.SuggestNext(current));
    }

    [Fact]
    public void SuggestNext_ReturnsNullForNull()
    {
        Assert.Null(VersionHelper.SuggestNext(null));
    }
}
