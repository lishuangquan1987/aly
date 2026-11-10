param(
    [string]$Configuration = "Release"
)

# Build the COM-visible wrapper (x86 / net40).
dotnet build $PSScriptRoot\AlyClient.Com.csproj -c $Configuration
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}