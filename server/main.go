// 审查 E2E 回归（用例 123）：
// Go 1.26 起 net/http 对 Range 的默认策略改为"最多只处理 1 段"（godebug
// httpservecontentmaxranges，Changed: 26 / Old: 1）。多段 Range 会被静默忽略、
// 回 200 全文——客户端若按 206 处理会把 MIME 边界写进 .part。
// 服务端明确需要 RFC 7233 的多段支持，这里显式恢复"最多 200 段"的阈值
// （保留 DoS 缓解：超过 200 段的请求仍按旧策略忽略）。
//go:debug httpservecontentmaxranges=200
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aly/server/controllers"
	"aly/server/internal/db"
	"aly/server/routers"

	"github.com/gin-gonic/gin"
)

func main() {

	//使用flag包
	var port int
	var dbPath string
	flag.IntVar(&port, "p", 2000, "监听的端口")
	flag.StringVar(&dbPath, "db", "", "数据库文件路径（默认程序根目录 aly.db）")
	flag.Parse()

	//初始化数据库
	if dbPath != "" {
		db.DBPath = dbPath
	}
	db.InitDB()
	defer db.Client.Close()

	// 审查 M3：启动时清扫超龄上传中间态（进程崩溃残留的 *.chunks / *.merging）
	if n, err := controllers.SweepStaleUploadTemps(24 * time.Hour); err != nil {
		log.Printf("清扫上传中间态失败（非致命）: %v", err)
	} else if n > 0 {
		log.Printf("已清扫 %d 个超龄上传中间态条目", n)
	}

	r := gin.Default()
	// 上传大小限制（#14）：multipart 内存阈值调小，超出部分落盘临时文件，
	// 配合 controllers.UploadFile/UploadChunk 的 MaxBytesReader + Content-Length 前置校验。
	r.MaxMultipartMemory = 8 << 20
	routers.InitRouter(r)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		// 审查 B11：WriteTimeout 是整个响应体的写出期限。原来固定 60s，
		// 1GB 文件需要 ≥17.4MB/s 才能发完，百兆网（12.5MB/s）必被服务端断链。
		// 这里置 0（无全局上限），改由各路由自行设置：
		//   - DownloadFile：按文件大小 SetWriteDeadline（含 30min 上限）
		//   - 普通 JSON 接口：JSONWriteDeadline 中间件给 60s
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器（非阻塞）
	go func() {
		fmt.Printf("启动中,监听端口:%d\r\n", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// 等待中断信号优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务器强制关闭: %v", err)
	}
	log.Println("服务器已退出")
}
