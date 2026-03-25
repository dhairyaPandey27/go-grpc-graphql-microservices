package catalog

import (
	"context"
	"log"

	"github.com/dhairyaPandey27/go-grpc-graphql-microservices/catalog/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn
	service pb.CatalogServiceClient
}

func NewClient(url string) (*Client,error){
	conn,err:=grpc.NewClient(url,grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err!=nil{
		log.Println(err)
		return nil,err
	}
	c:=pb.NewCatalogServiceClient(conn)
	return &Client{conn,c},nil
}

func (c *Client) Close(){
	c.conn.Close()
}

func (c *Client) PostProduct(ctx context.Context,name,description string, price float64) (*Product,error){
	r,err:=c.service.PostProduct(ctx,&pb.PostProductRequest{
		Name: name,
		Description: description,
		Price: price,
	})
	if err!=nil{
		return nil,err
	}

	return &Product{
		ID: r.Product.Id,
		Name: r.Product.Name,
		Description: r.Product.Description,
		Price: r.Product.Price,
	},nil
}


func (c *Client) GetProduct(ctx context.Context,id string) (*Product ,error){
	r,err:=c.service.GetProduct(ctx,&pb.GetProductRequest{
		Id: id,
	})
	if err!=nil{
		return nil,err
	}
	
	return &Product{
		ID: r.Product.Id,
		Name: r.Product.Name,
		Description: r.Product.Description,
		Price: r.Product.Price,
	},nil
}

func (c *Client) GetProducts (ctx context.Context,ids []string , query string,skip uint64,take uint64) ([]Product,error){
	r,err:=c.service.GetProducts(
		ctx,
		&pb.GetProductsRequest{
			Ids: ids,
			Skip: skip,
			Take: take,
			Query: query,
		},
	)
	if err!=nil{
		return nil,err
	}

	products := []Product{}
	for _,p:=range r.Product{
		products=append(products, Product{
			ID: p.Id,
			Name: p.Name,
			Description: p.Description,
			Price: p.Price,
		})
	}
	return products,nil
} 
