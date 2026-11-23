package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// JSONWriteDeadline 给普通 JSON 接口设置响应写期限（审查 B11）。
//
// 全局 WriteTimeout 已置 0（大文件下载不能有固定总时限），但普通 JSON 接口
// 响应体极小，没有写期限会让"客户端连上不读"的连接长期占用 goroutine。
// 下载路由不挂本中间件——它按文件大小自行设置期限。
func JSONWriteDeadline(d time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if rc := http.NewResponseController(ctx.Writer); rc != nil {
			_ = rc.SetWriteDeadline(time.Now().Add(d))
		}
		ctx.Next()
	}
}

// JSONMaxBytes 给 JSON 接口请求体设上限（code-review-2026-11-23 #5）。
//
// 上传链路已有 ContentLength 前置 + MaxBytesReader 三层限制，JSON 组此前裸奔：
// publish_version 的 logs 数组可提交数百 MB → gin 全量读入内存 → 原样写进
// SQLite。这与"无鉴权是有意设计"无关，属资源上限缺失。
// 超限时返回 NG（业务信封，HTTP 仍 200，与全站错误风格一致）。
func JSONMaxBytes(max int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Body != nil {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, max)
		}
		ctx.Next()
	}
}
