package main

import (
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	rootCmd = &cobra.Command{
		Use:   "todo",
		Short: "todo cli",
		Long:  `cli to manage yours todo lists`,
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	grpcClient todo.TodoListServiceClient
)

func execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func main() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient("127.0.0.1:50051", opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	grpcClient = todo.NewTodoListServiceClient(conn)

	execute()
}
