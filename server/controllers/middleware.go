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
