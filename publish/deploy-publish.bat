@echo off

chcp 65001 >nul

setlocal



set SCRIPT_DIR=%~dp0



echo ===== 编译 publish-cli =====

cd /d "%SCRIPT_DIR%publish-cli"

go build -o aly-publish.exe .

if %ERRORLEVEL% neq 0 (

    echo [失败] publish-cli 编译失败

    exit /b %ERRORLEVEL%

)

echo [成功] publish-cli 编译完成



echo ===== 复制 aly-publish.exe 到 publish-gui 输出目录 =====

copy /y "%SCRIPT_DIR%publish-cli\aly-publish.exe" "%SCRIPT_DIR%publish-gui\src\AlyPublish\bin\Debug\net8.0\" >nul

if %ERRORLEVEL% neq 0 (

    echo [失败] 复制 aly-publish.exe 失败

    exit /b %ERRORLEVEL%

)

echo [成功] 已复制到 publish-gui 输出目录



echo ===== 编译 publish-gui =====

cd /d "%SCRIPT_DIR%publish-gui\src\AlyPublish"

dotnet build -c Debug --no-restore

if %ERRORLEVEL% neq 0 (

    echo [失败] publish-gui 编译失败

    exit /b %ERRORLEVEL%

)

echo [成功] publish-gui 编译完成



echo ===== 全部完成 =====

endlocal

