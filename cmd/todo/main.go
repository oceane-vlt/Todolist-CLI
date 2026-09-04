package main

import (
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
	// No Run: a root command with subcommands and an empty Run silently prints
	// nothing when invoked bare. Leaving it out makes Cobra show the help, which
	// is the only useful answer to someone typing "todo" on its own.
	rootCmd = &cobra.Command{
		Use:   "todo",
		Short: "Manage your todo lists from the command line",
		Long: `todo manages todo lists from the command line.

Each item has a short title and an optional longer description. "todo show"
opens an interactive browser where you can read descriptions, tick items with x
and complete them with ctrl+s.`,
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
	// Cobra has already printed the error and the usage by the time Execute
	// returns, so printing it again here only duplicated it on screen. What was
	// missing is the exit status: a usage error used to leave the process at 0,
	// which made every failure invisible to a script or a CI step.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
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
		_ = conn.Close()
	}(conn)

	grpcClient = todo.NewTodoListServiceClient(conn)

	execute()
}
