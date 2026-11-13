@echo off
rem 本文件必须保持 CRLF 行尾：cmd 对 LF 行尾 + 中文注释会误拆行，
rem 把注释碎片当命令执行（报 "is not recognized as an internal ..."）。
rem
rem 构建链路：本机 go(1.27.3) 跑 deploy-client.go → 在 WSL 里用 Docker 镜像
rem 编出 windows/386 的 XP 兼容产物 → 校验 PE 头 → 回拷仓库并部署。
rem 前置：WSL 内 docker 可用，且编译镜像已存在（默认 golang:1.10.8-stretch；
rem 缺失时脚本会提示 docker pull，也可用 ALY_GO_IMAGE 指定别的 1.10.x 镜像）。
rem 不再依赖 WSL 内安装的 Go 1.10。
rem
rem 宿主编译必须落在本机默认目标（windows/amd64）。机器环境里若残留交叉
rem 编译变量 GOOS/GOARCH（如 GOOS=linux GOARCH=arm64），go 会按该目标产出
rem 不带 .exe 的产物，Windows 执行查找只认 .com/.exe/.bat/.cmd，于是报
rem "executable file not found in %PATH%"。这里清空，回退 go 自带默认值。
set GOOS=
set GOARCH=
rem GOROOT 只用于 go run 编译 deploy-client.go 本身（宿主 1.27.3 工具链）；
rem XP 产物由 Docker 镜像内的 go1.10.8 编译，与这里无关。
set GOROOT=D:\Install\go1.27.3
rem 产物工具链固定在这个镜像里；改成非 1.10.x 镜像会被脚本的版本断言拦住。
if not defined ALY_GO_IMAGE set ALY_GO_IMAGE=golang:1.10.8-stretch
cd /d "%~dp0"
go run "%~dp0deploy-client.go"
rem 先记住退出码再 pause：pause 会把 errorlevel 重置成 0，批处理必须以
rem go run 的结果退出，否则"构建/校验/部署失败"对调用方就是成功。
set RC=%ERRORLEVEL%
pause
exit /b %RC%