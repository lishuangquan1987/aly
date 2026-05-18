using CommunityToolkit.Mvvm.ComponentModel;
using Moq;
using PublishTool.Models.Local;
using PublishTool.Services;
using PublishTool.ViewModels;
using Shouldly;
using Xunit;

namespace PublishTool.Tests.ViewModels;

public class ProjectPageViewModelTests
{
    [Fact]
    public void AutoGenerateVersion_SetsCorrectFormat()
    {
        // Arrange
        var config = new ProjectConfig
        {
            ServerId = 1,
            Name = "test",
            Title = "Test",
            ServerUrl = "http://localhost:2000",
            LocalPath = @"C:\test"
        };
        var vm = new ProjectPageViewModel(
            config,
            Mock.Of<ProjectService>(),
            Mock.Of<FileService>(),
            Mock.Of<LocalFileService>(),
            Mock.Of<ProcessService>());

        // Act
        vm.AutoGenerateVersionCommand.Execute(null);

        // Assert
        vm.NewVersion.ShouldMatch(@"^\d{8}-\d{4}$");
    }

    [Fact]
    public void Constructor_InitializesPropertiesCorrectly()
    {
        // Arrange
        var config = new ProjectConfig
        {
            ServerId = 1,
            Name = "test",
            Title = "Test",
            ServerUrl = "http://localhost:2000",
            LocalPath = @"C:\test"
        };
        var vm = new ProjectPageViewModel(
            config,
            Mock.Of<ProjectService>(),
            Mock.Of<FileService>(),
            Mock.Of<LocalFileService>(),
            Mock.Of<ProcessService>());

        // Assert
        vm.Config.ShouldBe(config);
        vm.ServerVersion.ShouldBeEmpty();
        vm.StatusMessage.ShouldBeEmpty();
        vm.IsRefreshEnabled.ShouldBeTrue();
    }
}
