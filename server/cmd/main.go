package main

import (
	"flag"
	"fmt"

	"clientupdator/server/internal/db"
	"clientupdator/server/routers"

	"github.com/gin-gonic/gin"
)

func main() {

	//初始化数据库
	db.InitDB()
	defer db.Client.Close()

	r := gin.Default()
	routers.InitRouter(r)
	//使用flag包
	var port int
	flag.IntVar(&port, "p", 2000, "监听的端口")
	flag.Parse()

	fmt.Printf("启动中,监听端口:%d\r\n", port)

	panic(r.Run(fmt.Sprintf(":%d", port)))
}
