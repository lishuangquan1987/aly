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

	r := gin.Default()
	routers.InitRouter(r)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
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
