package main

type AppConfig struct{
	AccountURL string `envconfig:"ACCOUNT_SERVICE_URL"`
	CatalogURL string `envconfig:"CATALOG_SERVICE_URL"`
	OrderURL string `envconfig:"ORDER_SERVICE_URL"`
}

func main(){

}
