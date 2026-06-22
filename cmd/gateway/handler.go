package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	todo "github.com/oceane-vlt/todolist/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// authHeaderKey is the incoming HTTP header carrying the bearer token.
const authHeaderKey = "Authorization"

// grpcAuthMetadataKey is the gRPC metadata key the server's auth interceptor
// reads (server/authinterceptor.go: "authorization"). gRPC lowercases metadata
// keys; we set it lowercase explicitly.
const grpcAuthMetadataKey = "authorization"

// marshaler emits proto3-canonical JSON, the same wire format a generated
// grpc-gateway stub would produce, so the REST surface stays compatible with the
// canonical generated path (docs/grpc-gateway.md, volet B).
var marshaler = protojson.MarshalOptions{
	UseProtoNames:   false, // camelCase field names (proto3 JSON default).
	EmitUnpopulated: true,  // stable shape for empty responses.
}

var unmarshaler = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

// newHandler builds the REST/JSON router mapping the seven RPCs to HTTP routes.
// It is decoupled from the transport (it takes a gRPC client interface) so it
// can be exercised against an in-memory server in tests.
//
// The handler is a pure relay: it forwards the inbound "Authorization: Bearer"
// header to the upstream gRPC call as "authorization" metadata so the server's
// auth interceptor (Phase 3) derives the user_id and enforces isolation. The
// gateway never validates the token itself.
func newHandler(client todo.TodoListServiceClient) http.Handler {
	mux := http.NewServeMux()

	// Collection of lists.
	mux.HandleFunc("POST /v1/lists", func(w http.ResponseWriter, r *http.Request) {
		var req todo.CreateTodoListRequest
		if err := decode(r, &req); err != nil {
			writeError(w, status.Error(codes.InvalidArgument, err.Error()))
			return
		}
		resp, err := client.CreateTodoList(callContext(r), &req)
		writeResult(w, resp, err)
	})

	mux.HandleFunc("GET /v1/lists", func(w http.ResponseWriter, r *http.Request) {
		resp, err := client.GetTodoLists(callContext(r), &todo.GetTodoListsRequest{})
		writeResult(w, resp, err)
	})

	mux.HandleFunc("DELETE /v1/lists", func(w http.ResponseWriter, r *http.Request) {
		var req todo.DeleteTodoListRequest
		if err := decode(r, &req); err != nil {
			writeError(w, status.Error(codes.InvalidArgument, err.Error()))
			return
		}
		resp, err := client.DeleteTodoList(callContext(r), &req)
		writeResult(w, resp, err)
	})

	// Items of a single list. The list title is taken from the path so the URL
	// reads naturally; the request body carries the rest.
	mux.HandleFunc("GET /v1/lists/{title}/items", func(w http.ResponseWriter, r *http.Request) {
		req := todo.ShowTodoListItemsRequest{Title: pathTitle(r)}
		resp, err := client.ShowTodoListItems(callContext(r), &req)
		writeResult(w, resp, err)
	})

	mux.HandleFunc("PUT /v1/lists/{title}/items", func(w http.ResponseWriter, r *http.Request) {
		var req todo.UpdateTodoListRequest
		if err := decode(r, &req); err != nil {
			writeError(w, status.Error(codes.InvalidArgument, err.Error()))
			return
		}
		req.Title = pathTitle(r)
		resp, err := client.UpdateTodoList(callContext(r), &req)
		writeResult(w, resp, err)
	})

	mux.HandleFunc("PATCH /v1/lists/{title}/items", func(w http.ResponseWriter, r *http.Request) {
		var req todo.UpdateTodoListItemRequest
		if err := decode(r, &req); err != nil {
			writeError(w, status.Error(codes.InvalidArgument, err.Error()))
			return
		}
		req.Title = pathTitle(r)
		resp, err := client.UpdateTodoListItem(callContext(r), &req)
		writeResult(w, resp, err)
	})

	mux.HandleFunc("DELETE /v1/lists/{title}/items", func(w http.ResponseWriter, r *http.Request) {
		var req todo.DeleteTodoListItemsRequest
		if err := decode(r, &req); err != nil {
			writeError(w, status.Error(codes.InvalidArgument, err.Error()))
			return
		}
		req.Title = pathTitle(r)
		resp, err := client.DeleteTodoListItems(callContext(r), &req)
		writeResult(w, resp, err)
	})

	return mux
}

// pathTitle returns the {title} path variable, unescaped by the router.
func pathTitle(r *http.Request) string {
	return r.PathValue("title")
}

// callContext builds the outgoing gRPC context, forwarding the inbound bearer
// token as "authorization" metadata when present. This is the core of the
// gateway: the HTTP Authorization header becomes gRPC metadata so the server's
// auth interceptor can derive the user identity (header -> metadata).
func callContext(r *http.Request) context.Context {
	ctx := r.Context()
	if header := r.Header.Get(authHeaderKey); header != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, grpcAuthMetadataKey, header)
	}
	return ctx
}

// decode reads a proto message from the request body as proto3-canonical JSON.
// An empty body is accepted and leaves the message at its zero value, which is
// convenient for requests whose only input comes from the path.
func decode(r *http.Request, msg proto.Message) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	return unmarshaler.Unmarshal(body, msg)
}

// maxBodyBytes caps request bodies to a sane size for a todo payload.
const maxBodyBytes = 1 << 20 // 1 MiB

// writeResult writes a successful proto response as JSON, or maps a gRPC error
// to the corresponding HTTP status.
func writeResult(w http.ResponseWriter, resp proto.Message, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	data, marshalErr := marshaler.Marshal(resp)
	if marshalErr != nil {
		writeError(w, status.Error(codes.Internal, marshalErr.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// writeError maps a gRPC status error to an HTTP status code and a JSON error
// body. Unauthenticated -> 401 is the signal a web front would use to
// re-authenticate, mirroring how the CLI treats it (refresh-and-replay).
func writeError(w http.ResponseWriter, err error) {
	code := codes.Unknown
	msg := err.Error()
	if st, ok := status.FromError(err); ok {
		code = st.Code()
		msg = st.Message()
	}

	httpCode := httpStatusFromGRPC(code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:    code.String(),
		Message: msg,
	})
}

// errorBody is the JSON error envelope returned to HTTP clients.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// httpStatusFromGRPC maps gRPC status codes to HTTP status codes, following the
// same correspondence grpc-gateway uses so the canonical generated path would
// behave identically.
func httpStatusFromGRPC(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Canceled:
		return 499 // client closed request (nginx convention, as grpc-gateway).
	default:
		return http.StatusInternalServerError
	}
}
