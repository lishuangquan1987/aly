using AlyPublish.Helpers;
using Xunit;

namespace AlyPublish.Tests;

public sealed class ListDiffHelperTests
{
    [Fact]
    public void Compute_ReturnsAddedAndRemovedItems()
    {
        var result = ListDiffHelper.Compute(["a", "b"], ["b", "c"]);

        Assert.Equal(["c"], result.Added);
        Assert.Equal(["a"], result.Removed);
    }

    [Fact]
    public void Compute_RemovesDuplicatesAndKeepsOrder()
    {
        var result = ListDiffHelper.Compute(["a"], ["c", "c", "b"]);

        Assert.Equal(["c", "b"], result.Added);
        Assert.Equal(["a"], result.Removed);
    }

    [Fact]
    public void Compute_IsCaseSensitive()
    {
        var result = ListDiffHelper.Compute(["Bin"], ["bin"]);

        Assert.Equal(["bin"], result.Added);
        Assert.Equal(["Bin"], result.Removed);
    }

    [Fact]
    public void Compute_IdenticalListsProduceNoChanges()
    {
        var result = ListDiffHelper.Compute(["a", "b"], ["a", "b"]);

        Assert.Empty(result.Added);
        Assert.Empty(result.Removed);
    }
}
