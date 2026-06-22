// Command gateway exposes the existing TodoList gRPC service over REST/JSON for a
// future web front (docs/implementation-plan.md Phase 7, docs/target-architecture.md §3).
//
// It is a thin, hand-written REST <-> gRPC adapter: it dials the gRPC server as
// a client, reusing the already-generated TodoListServiceClient, and maps the
// seven RPCs to HTTP/JSON routes. It forwards the inbound
// "Authorization: Bearer <jwt>" header as gRPC "authorization" metadata so the
// server's auth interceptor (Phase 3) derives the user identity — the gateway
// itself never validates tokens. The canonical generated grpc-gateway path
// (google.api.http annotations + buf) is documented in docs/grpc-gateway.md for
// when the protoc/buf toolchain is available; this binary keeps the project
// building and the REST surface working today without that toolchain.
package main

import (
	"log"
	"net/http"
	"time"

	todo "github.com/oceane-vlt/todolist/proto"
	"google.golang.org/grpc"
)

func main() {
	if err := validateConfig(); err != nil {
		log.Fatal(err)
	}

	endpoint := grpcEndpoint()
	conn, err := grpc.NewClient(endpoint, upstreamCredentials())
	if err != nil {
		log.Fatalf("dialing gRPC server %s: %v", endpoint, err)
	}
	defer func() { _ = conn.Close() }()

	client := todo.NewTodoListServiceClient(conn)
	handler := newHandler(client)

	addr := httpAddr()
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("REST/JSON gateway listening on %s, forwarding to gRPC %s", addr, endpoint)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
