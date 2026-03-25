package account

import (
	"context"
	"fmt"
	"net"

	"github.com/dhairyaPandey27/go-grpc-graphql-microservices/account/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	service Service
	pb.UnimplementedAccountServiceServer
}

func ListenGRPC(s Service,port int64) error{
	lis,err:=net.Listen("tcp",fmt.Sprintf(":%d",port))
	if err!=nil{
		return err
	}
	serv:=grpc.NewServer()
	pb.RegisterAccountServiceServer(serv,&grpcServer{
		UnimplementedAccountServiceServer: pb.UnimplementedAccountServiceServer{},
		service: s,
	})
	reflection.Register(serv)
	return serv.Serve(lis)
}

func (s *grpcServer) PostAccount(ctx context.Context,r *pb.PostAcccountRequest) (*pb.PostAcccountResponse,error){
	a,err:=s.service.PostAccount(ctx,r.Name)
	if err!=nil{
		return nil,err
	}
	return &pb.PostAcccountResponse{Account:&pb.Account{
		Id: a.ID,
		Name: a.Name,
	}},nil
}

func (s *grpcServer) GetAccount(ctx context.Context,r *pb.GetAccountRequest) (*pb.GetAccountResponse,error){
	a,err:=s.service.GetAccount(ctx,r.Id)
	if err!=nil{
		return nil,err
	}

	return &pb.GetAccountResponse{Account:&pb.Account{
		Id: a.ID,
		Name: a.Name,
	}},nil
}

func (s *grpcServer) GetAccounts(ctx context.Context,r *pb.GetAccountsRequest) (*pb.GetAccountsResponse,error){
	a,err:=s.service.GetAccounts(ctx,r.Skip,r.Take)
	if err!=nil{
		return nil,err
	}

	Accounts:=[]*pb.Account{}
	for _,p:=range a{
		Accounts=append(Accounts, &pb.Account{
			Id: p.ID,
			Name: p.Name,
		})
	}
	return &pb.GetAccountsResponse{Account: Accounts},nil
}
