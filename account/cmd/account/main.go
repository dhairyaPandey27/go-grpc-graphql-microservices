package main 

import (
	"log"
	"time"

	"github.com/dhairyaPandey27/go-grpc-graphql-microservices/account"
	"github.com/tinrab/retry"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DatabaseURl string `envconfig:"DATABASE_URL"`
}

func main() {
	var cfg Config
	err := envconfig.Process("",&cfg)
	if err!=nil{
		log.Fatal(err)
	}
	
	var r account.Repository
	retry.ForeverSleep(2*time.Second,func(_ int)(err error){
		r,err=account.NewPostgresRepository(cfg.DatabaseURl)
		if err!=nil{
			log.Fatal(err)
		}
		return
	})

	defer r.Close()
	log.Println("(Account - main.go) Listening on port 8080....")
	s:=account.NewService(r)
	log.Fatal(account.ListenGRPC(s,8080))
}
