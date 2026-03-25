package main

import (
	"log"
	"time"

	"github.com/dhairyaPandey27/go-grpc-graphql-microservices/catalog"
	"github.com/kelseyhightower/envconfig"
	"github.com/tinrab/retry"
)

type config struct {
	DatabaseURL string `envconfig:"DATABASE_URL"`
}

func main() {
	var cfg config
	err := envconfig.Process("",&cfg)
	if err!=nil{
		log.Fatal(err)
	}

	var r catalog.Repository
	retry.ForeverSleep(2*time.Second,func(_ int)(err error){
		r,err=catalog.NewElasticRepository(cfg.DatabaseURL)
		if err!=nil{
			log.Println(err)
		}
		return
	})
	defer r.Close()
	log.Println("(Catalog - main.go) Listening on port 8080....")
	s:=catalog.NewService(r)
	log.Fatal(catalog.ListenGRPC(s,8080))
}
