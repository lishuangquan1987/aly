using AlyPublish.Services;
using Xunit;

namespace AlyPublish.Tests;

public sealed class PushResultParserTests
{
    [Fact]
    public void Parse_ProgressStart_ReturnsProgress()
    {
        var line = """{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":3,"file":"a.dll","status":"START","file_size":12}}""";

        var result = PushResultParser.Parse(line);

        Assert.Equal(PushLineKind.Progress, result.Kind);
        Assert.NotNull(result.Progress);
        Assert.Equal("a.dll", result.Progress.File);
        Assert.Null(result.ErrorMessage);
    }

    [Fact]
    public void Parse_ProgressFail_KeepsFailureReason()
    {
        var line = """{"isSuccess":false,"errorMsg":"a.dll: timeout","data":{"index":1,"total":3,"file":"a.dll","status":"FAIL","file_size":12,"error":"timeout"}}""";

        var result = PushResultParser.Parse(line);

        Assert.Equal(PushLineKind.Progress, result.Kind);
        Assert.Equal("a.dll: timeout", result.ErrorMessage);
    }

    [Fact]
    public void Parse_FinalSuccess_ReturnsSuccess()
    {
        var line = """{"isSuccess":true,"errorMsg":"","data":{"version":"V1.0.1","files":2}}""";

        var result = PushResultParser.Parse(line);

        Assert.Equal(PushLineKind.Success, result.Kind);
    }

    [Fact]
    public void Parse_FailureResultWithNullData_ReturnsFailure()
    {
        var line = """{"isSuccess":false,"errorMsg":"版本号已存在","data":null}""";

        var result = PushResultParser.Parse(line);

        Assert.Equal(PushLineKind.Failure, result.Kind);
        Assert.Equal("版本号已存在", result.ErrorMessage);
    }

    [Fact]
    public void Parse_ProgressDoneSentinel_IsIgnored()
    {
        var line = """{"isSuccess":true,"errorMsg":"","data":null}""";

        var result = PushResultParser.Parse(line);

        Assert.Equal(PushLineKind.Ignored, result.Kind);
    }

    [Fact]
    public void Parse_NonJsonLine_IsIgnored()
    {
        var result = PushResultParser.Parse("starting upload...");

        Assert.Equal(PushLineKind.Ignored, result.Kind);
    }
}
