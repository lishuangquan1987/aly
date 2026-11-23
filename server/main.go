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
	var printRoutes bool
	flag.IntVar(&port, "p", 2000, "监听的端口")
	flag.StringVar(&dbPath, "db", "", "数据库文件路径（默认程序根目录 aly.db）")
	// 文档防漂移：只打印路由清单（METHOD PATH）就退出，不碰数据库、不监听；
	// website/scripts/check_docs.py 用它核对网站 API 文档。
	flag.BoolVar(&printRoutes, "print-routes", false, "打印全部路由后退出（供文档核对脚本使用）")
	flag.Parse()

	if printRoutes {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		routers.InitRouter(r)
		for _, route := range r.Routes() {
			fmt.Printf("%s %s\n", route.Method, route.Path)
		}
		return
	}

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
		// 审查修复（轻微⑧）：60s 对 64MB 上限的分片偏紧（64MB/60s ≈ 11MB/s，
		// 慢链路必掐）。提到 300s——分片默认 128KiB 不受影响，仅 ALY_CHUNK_SIZE
		// 调大时才依赖这个余量。
		ReadTimeout:  300 * time.Second,
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
