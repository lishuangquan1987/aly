package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	if err := entc.Generate("./ent/schema", &gen.Config{}); err != nil {
		log.Fatalf("generate ent code error:%v", err)
	}
	log.Println("generate ent code success")
}
