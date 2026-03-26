package main

import (
	"log"
	"time"

	"github.com/dhairyaPandey27/go-grpc-graphql-microservices/order"
	"github.com/kelseyhightower/envconfig"
	"github.com/tinrab/retry"
)

type config struct {
	DatabaseURL string `envconfig:"DATABASE_URL"`
	AccountURL  string `envconfig:"ACCOUNT_SERVICES_URL"`
	CatalogURL  string `envconfig:"CATALOG_SERVICE_URL"`
}

func main() {
	var cfg config
	err := envconfig.Process("",&cfg)
	if err!=nil{
		log.Fatal(err)
	}


	var r order.Repository
	retry.ForeverSleep(2*time.Second,func(_ int) (err error) {
		r,err=order.NewPostgresRepository(cfg.DatabaseURL)
		if err!=nil{
			log.Println(err)
		}
		return
	})
	defer r.Close()
	log.Println("Listening on port 8080....")
	s:=order.NewService(r)
	log.Fatal(order.ListenGRPC(s,cfg.AccountURL,cfg.CatalogURL,8080))
}
