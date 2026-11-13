using AlyPublish.Helpers;
using Xunit;

namespace AlyPublish.Tests;

public sealed class FileSizeHelperTests
{
    [Theory]
    [InlineData(0, "0 B")]
    [InlineData(1023, "1023 B")]
    [InlineData(1024, "1 KB")]
    [InlineData(1536, "1.5 KB")]
    [InlineData(1_572_864, "1.5 MB")]
    [InlineData(1024L * 1024 * 1024, "1 GB")]
    public void Format_UsesBinaryUnits(long bytes, string expected)
    {
        Assert.Equal(expected, FileSizeHelper.Format(bytes));
    }

    [Fact]
    public void Format_NegativeValueFallsBackToZero()
    {
        Assert.Equal("0 B", FileSizeHelper.Format(-1));
    }
}
