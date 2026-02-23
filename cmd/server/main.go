package main

import (
	"log"
	"net"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/oceane-vlt/todolist/server"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", "127.0.0.1:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	todo.RegisterTodoListServiceServer(grpcServer, &server.TodoListServer{})

	log.Println("gRPC server listening on 127.0.0.1:50050")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
