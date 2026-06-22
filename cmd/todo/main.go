package main

import (
	"fmt"
	"log"
	"os"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// envServerEndpoint overrides the gRPC server address the CLI dials. It lets the
// CLI target a remote server instead of the hard-coded local one. The TLS
// transport that secures that remote connection is configured separately (see
// tlsconfig.go, Phase 4).
const envServerEndpoint = "TODO_SERVER_ENDPOINT"

// defaultServerEndpoint is the local server used when TODO_SERVER_ENDPOINT is
// unset, preserving today's behaviour.
const defaultServerEndpoint = "127.0.0.1:50051"

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

// serverEndpoint resolves the server address from the environment, falling back
// to the local default.
func serverEndpoint() string {
	if endpoint := os.Getenv(envServerEndpoint); endpoint != "" {
		return endpoint
	}
	return defaultServerEndpoint
}

func execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func main() {
	// Transport security (Phase 4): TLS when configured (TODO_TLS /
	// TODO_TLS_CA_FILE), insecure otherwise to preserve the local default.
	transportCreds, err := transportCredentials()
	if err != nil {
		log.Fatal(err)
	}

	// The timeout interceptor bounds every call so a slow/unreachable remote
	// server fails cleanly; the auth interceptor attaches the bearer token and
	// refreshes it on Unauthenticated (Phase 3). The timeout runs first so its
	// deadline also covers a refresh-and-replay.
	opts := []grpc.DialOption{
		transportCreds,
		grpc.WithChainUnaryInterceptor(timeoutUnaryInterceptor, authUnaryInterceptor),
	}

	conn, err := grpc.NewClient(serverEndpoint(), opts...)
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
