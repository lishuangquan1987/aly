@echo off
rem 本文件必须保持 CRLF 行尾：cmd 对 LF 行尾 + 中文注释会误拆行，
rem 把注释碎片当命令执行（报 "is not recognized as an internal ..."）。
rem
rem 宿主编译必须落在本机默认目标（windows/amd64）。机器环境里若残留交叉
rem 编译变量 GOOS/GOARCH（如 GOOS=linux GOARCH=arm64），go 会按该目标产出
rem 不带 .exe 的产物，Windows 执行查找只认 .com/.exe/.bat/.cmd，于是报
rem "executable file not found in %PATH%"。这里清空，回退 go 自带默认值。
rem 交叉编译目标由 deploy-server.go 内部显式设定，与宿主环境无关。
set GOOS=
set GOARCH=
rem GOROOT must point at the toolchain that actually compiles this machine:
rem the windows-cross builds run on the 1.27.3 toolchain, so its root must be used
rem (leaving the 1.10.8 root makes its own compile report "-lang" per package).
set GOROOT=D:\Install\go1.27.3
cd /d "%~dp0"
go run "%~dp0deploy-server.go" %*
pause
