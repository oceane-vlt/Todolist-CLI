package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/oceane-vlt/todolist/libs/storage"
	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/oceane-vlt/todolist/server"
	"google.golang.org/grpc"
)

// envDevUserID overrides the development user identity injected into every
// request context (Phase 2). Defaults to storage.PlaceholderUserID. This is a
// transition stand-in replaced by real JWT-derived identity in Phase 3.
const envDevUserID = "TODO_DEV_USER_ID"

func main() {
	// Phase 6: the listen address is configurable so the server can be hosted
	// publicly on a PaaS (Fly.io injects PORT -> bind 0.0.0.0:$PORT). Without
	// any configuration it stays on 127.0.0.1:50051, so the default local run is
	// unchanged (docs/target-architecture.md §6.2).
	listenAddr := server.ListenAddrFromEnv()
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// The local JSON data file is always prepared so the default (TODO_STORAGE
	// unset/json) keeps working with no configuration. The Postgres backend
	// (TODO_STORAGE=postgres) ignores this path and uses DATABASE_URL instead.
	dataPath, err := server.DefaultDataFilePath()
	if err != nil {
		log.Fatalf("failed to initialize data file: %v", err)
	}

	store, closeStore, err := storage.NewStoreFromEnv(context.Background(), dataPath)
	if err != nil {
		log.Fatalf("failed to initialize storage backend: %v", err)
	}
	defer closeStore()

	// Phase 3: authenticate the caller and inject the real per-request user_id
	// into the context (the seam introduced in Phase 2). When JWT_SIGNING_KEY or
	// SUPABASE_JWT_SECRET is set, the JWT auth interceptor validates the bearer
	// token. Otherwise we fall back to the Phase 2 dev interceptor with a fixed
	// identity (placeholder by default, overridable via TODO_DEV_USER_ID) so the
	// default local run keeps working with no auth configuration.
	devUserID := os.Getenv(envDevUserID)
	if devUserID == "" {
		devUserID = storage.PlaceholderUserID
	}

	// When the backend is Postgres, pass the store as the user provisioner so the
	// auth interceptor provisions the authenticated user just-in-time (the FK
	// target of lists.user_id). The JSON backend has no users table -> nil
	// provisioner -> behaviour unchanged.
	var provisioner server.UserProvisioner
	if pg, ok := store.(*storage.PgStore); ok {
		provisioner = pg
	}

	interceptor, authEnabled := server.AuthInterceptorFromEnv(devUserID, provisioner)
	if authEnabled {
		log.Println("authentication enabled (JWT bearer token required)")
	} else {
		log.Printf("authentication disabled (dev mode, user_id=%s)", devUserID)
	}

	// Phase 4: serve over TLS when TLS_CERT_FILE and TLS_KEY_FILE are set,
	// ending the insecure transport. Without them the server stays insecure so
	// the default local run keeps working with no configuration (and so the PaaS
	// can terminate TLS in front of it, cf. docs/target-architecture.md §6.2).
	tlsOpt, tlsEnabled, err := server.TransportCredentialsFromEnv()
	if err != nil {
		log.Fatalf("failed to configure transport security: %v", err)
	}
	if tlsEnabled {
		log.Println("transport security enabled (TLS)")
	} else {
		log.Println("transport security disabled (insecure)")
	}

	opts := []grpc.ServerOption{
		tlsOpt,
		grpc.UnaryInterceptor(interceptor),
	}
	grpcServer := grpc.NewServer(opts...)

	todo.RegisterTodoListServiceServer(grpcServer, server.NewTodoListServer(store))

	log.Printf("gRPC server listening on %s", listenAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
