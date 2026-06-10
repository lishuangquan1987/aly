package main

import (
	"flag"
	"fmt"
	"log"

	"zap/server/internal/db"
	"zap/server/routers"

	"github.com/gin-gonic/gin"
)

func main() {

	//使用flag包
	var port int
	var dbPath string
	flag.IntVar(&port, "p", 2000, "监听的端口")
	flag.StringVar(&dbPath, "db", "", "数据库文件路径（默认程序根目录 zap.db）")
	flag.Parse()

	//初始化数据库
	if dbPath != "" {
		db.DBPath = dbPath
	}
	db.InitDB()
	defer db.Client.Close()

	r := gin.Default()
	routers.InitRouter(r)

	fmt.Printf("启动中,监听端口:%d\r\n", port)

	log.Fatal(r.Run(fmt.Sprintf(":%d", port)))
}
