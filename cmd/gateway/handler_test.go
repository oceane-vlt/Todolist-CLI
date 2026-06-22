package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	todo "github.com/oceane-vlt/todolist/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeService is an in-memory TodoListService used to exercise the gateway end
// to end over a real (in-process) gRPC connection. It records the "authorization"
// metadata it observed so tests can assert the HTTP Authorization header was
// forwarded verbatim as gRPC metadata, and lets each test inject a canned
// response or error.
type fakeService struct {
	todo.UnimplementedTodoListServiceServer

	lastAuthMetadata string

	createResp *todo.CreateTodoListResponse
	getResp    *todo.GetTodoListsResponse
	showResp   *todo.ShowTodoListItemsResponse
	deleteResp *todo.DeleteTodoListResponse
	err        error

	// lastShowTitle / lastUpdateItemReq / lastUpdateListReq / lastDeleteItemsReq
	// capture inputs for path-routing assertions.
	lastShowTitle      string
	lastUpdateItemReq  *todo.UpdateTodoListItemRequest
	lastUpdateListReq  *todo.UpdateTodoListRequest
	lastDeleteItemsReq *todo.DeleteTodoListItemsRequest
}

func (f *fakeService) recordAuth(ctx context.Context) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(grpcAuthMetadataKey); len(vals) > 0 {
			f.lastAuthMetadata = vals[0]
		}
	}
}

func (f *fakeService) CreateTodoList(ctx context.Context, _ *todo.CreateTodoListRequest) (*todo.CreateTodoListResponse, error) {
	f.recordAuth(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.createResp, nil
}

func (f *fakeService) GetTodoLists(ctx context.Context, _ *todo.GetTodoListsRequest) (*todo.GetTodoListsResponse, error) {
	f.recordAuth(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.getResp, nil
}

func (f *fakeService) ShowTodoListItems(ctx context.Context, req *todo.ShowTodoListItemsRequest) (*todo.ShowTodoListItemsResponse, error) {
	f.recordAuth(ctx)
	f.lastShowTitle = req.GetTitle()
	if f.err != nil {
		return nil, f.err
	}
	return f.showResp, nil
}

func (f *fakeService) DeleteTodoList(ctx context.Context, _ *todo.DeleteTodoListRequest) (*todo.DeleteTodoListResponse, error) {
	f.recordAuth(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.deleteResp, nil
}

func (f *fakeService) UpdateTodoListItem(ctx context.Context, req *todo.UpdateTodoListItemRequest) (*todo.UpdateTodoListItemResponse, error) {
	f.recordAuth(ctx)
	f.lastUpdateItemReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &todo.UpdateTodoListItemResponse{}, nil
}

func (f *fakeService) UpdateTodoList(ctx context.Context, req *todo.UpdateTodoListRequest) (*todo.UpdateTodoListResponse, error) {
	f.recordAuth(ctx)
	f.lastUpdateListReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &todo.UpdateTodoListResponse{}, nil
}

func (f *fakeService) DeleteTodoListItems(ctx context.Context, req *todo.DeleteTodoListItemsRequest) (*todo.DeleteTodoListItemsResponse, error) {
	f.recordAuth(ctx)
	f.lastDeleteItemsReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &todo.DeleteTodoListItemsResponse{}, nil
}

// newTestGateway spins up the fake gRPC service over an in-memory bufconn
// listener, dials it with a real gRPC client, and wraps the gateway handler in
// an httptest server. Everything is torn down via t.Cleanup.
func newTestGateway(t *testing.T, svc *fakeService) (*httptest.Server, *fakeService) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	todo.RegisterTodoListServiceServer(grpcServer, svc)
	go func() { _ = grpcServer.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}

	client := todo.NewTodoListServiceClient(conn)
	httpSrv := httptest.NewServer(newHandler(client))

	t.Cleanup(func() {
		httpSrv.Close()
		_ = conn.Close()
		grpcServer.Stop()
		_ = lis.Close()
	})

	return httpSrv, svc
}

// doRequest issues an HTTP request against the gateway, optionally with an
// Authorization header and a body, and returns the response.
func doRequest(t *testing.T, srv *httptest.Server, method, path, auth, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// TestGatewayForwardsAuthorizationHeaderToMetadata is the core assertion of the
// gateway: the HTTP Authorization header must arrive verbatim as the gRPC
// "authorization" metadata the server's auth interceptor reads, so the user
// identity is derived server-side (header -> metadata).
func TestGatewayForwardsAuthorizationHeaderToMetadata(t *testing.T) {
	svc := &fakeService{getResp: &todo.GetTodoListsResponse{}}
	srv, fake := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodGet, "/v1/lists", "Bearer test-jwt-123", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if fake.lastAuthMetadata != "Bearer test-jwt-123" {
		t.Fatalf("forwarded authorization metadata = %q, want %q", fake.lastAuthMetadata, "Bearer test-jwt-123")
	}
}

func TestGatewayWithoutAuthorizationSendsNoMetadata(t *testing.T) {
	svc := &fakeService{getResp: &todo.GetTodoListsResponse{}}
	srv, fake := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodGet, "/v1/lists", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if fake.lastAuthMetadata != "" {
		t.Fatalf("authorization metadata = %q, want empty when no header sent", fake.lastAuthMetadata)
	}
}

func TestGatewayGetListsReturnsJSON(t *testing.T) {
	svc := &fakeService{getResp: &todo.GetTodoListsResponse{
		Lists: []*todo.ListSize{{Title: "work", Size: 2}},
	}}
	srv, _ := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodGet, "/v1/lists", "Bearer x", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var body struct {
		Lists []struct {
			Title string `json:"title"`
			Size  int32  `json:"size"`
		} `json:"lists"`
	}
	decodeJSON(t, resp, &body)
	if len(body.Lists) != 1 || body.Lists[0].Title != "work" || body.Lists[0].Size != 2 {
		t.Fatalf("body = %+v, want one list work/2", body)
	}
}

func TestGatewayCreateDecodesBody(t *testing.T) {
	svc := &fakeService{createResp: &todo.CreateTodoListResponse{}}
	srv, _ := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodPost, "/v1/lists", "Bearer x",
		`{"title":"groceries","item":[{"title":"milk"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestGatewayShowItemsUsesPathTitle verifies the {title} path variable is wired
// into the gRPC request.
func TestGatewayShowItemsUsesPathTitle(t *testing.T) {
	svc := &fakeService{showResp: &todo.ShowTodoListItemsResponse{
		Items: []*todo.Item{{Title: "milk", Completed: false}},
	}}
	srv, fake := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodGet, "/v1/lists/groceries/items", "Bearer x", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if fake.lastShowTitle != "groceries" {
		t.Fatalf("show title = %q, want %q (from path)", fake.lastShowTitle, "groceries")
	}
}

// TestGatewayPatchItemPathOverridesBodyTitle verifies the path {title} wins over
// any title in the body (matching the handler which sets req.Title = pathTitle).
func TestGatewayPatchItemPathOverridesBodyTitle(t *testing.T) {
	svc := &fakeService{}
	srv, fake := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodPatch, "/v1/lists/groceries/items", "Bearer x",
		`{"title":"ignored","itemIndex":1,"newTitle":"bread"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if fake.lastUpdateItemReq.GetTitle() != "groceries" {
		t.Fatalf("title = %q, want %q (path wins over body)", fake.lastUpdateItemReq.GetTitle(), "groceries")
	}
	if fake.lastUpdateItemReq.GetNewTitle() != "bread" {
		t.Fatalf("newTitle = %q, want %q (from body)", fake.lastUpdateItemReq.GetNewTitle(), "bread")
	}
}

// TestGatewayPutListUsesPathTitleAndBodyItems verifies the PUT route
// (UpdateTodoList) wires the {title} path variable into the gRPC request and
// decodes the items from the body.
func TestGatewayPutListUsesPathTitleAndBodyItems(t *testing.T) {
	svc := &fakeService{}
	srv, fake := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodPut, "/v1/lists/groceries/items", "Bearer x",
		`{"title":"ignored","items":[{"title":"milk"},{"title":"bread"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if fake.lastUpdateListReq.GetTitle() != "groceries" {
		t.Fatalf("title = %q, want %q (path wins over body)", fake.lastUpdateListReq.GetTitle(), "groceries")
	}
	items := fake.lastUpdateListReq.GetItems()
	if len(items) != 2 || items[0].GetTitle() != "milk" || items[1].GetTitle() != "bread" {
		t.Fatalf("items = %+v, want milk,bread (from body)", items)
	}
}

// TestGatewayDeleteItemsUsesPathTitleAndBodyIndexes verifies the DELETE items
// route (DeleteTodoListItems) wires the {title} path variable and decodes the
// item indexes from the body.
func TestGatewayDeleteItemsUsesPathTitleAndBodyIndexes(t *testing.T) {
	svc := &fakeService{}
	srv, fake := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodDelete, "/v1/lists/groceries/items", "Bearer x",
		`{"title":"ignored","itemIndexes":[0,2]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if fake.lastDeleteItemsReq.GetTitle() != "groceries" {
		t.Fatalf("title = %q, want %q (path wins over body)", fake.lastDeleteItemsReq.GetTitle(), "groceries")
	}
	idx := fake.lastDeleteItemsReq.GetItemIndexes()
	if len(idx) != 2 || idx[0] != 0 || idx[1] != 2 {
		t.Fatalf("itemIndexes = %v, want [0 2] (from body)", idx)
	}
}

// TestGatewayMapsGRPCErrorsToHTTP checks the gRPC-status -> HTTP-status mapping
// for the codes a web front cares about most.
func TestGatewayMapsGRPCErrorsToHTTP(t *testing.T) {
	tests := []struct {
		name     string
		code     codes.Code
		wantHTTP int
	}{
		{"unauthenticated -> 401", codes.Unauthenticated, http.StatusUnauthorized},
		{"not found -> 404", codes.NotFound, http.StatusNotFound},
		{"already exists -> 409", codes.AlreadyExists, http.StatusConflict},
		{"invalid argument -> 400", codes.InvalidArgument, http.StatusBadRequest},
		{"unavailable -> 503", codes.Unavailable, http.StatusServiceUnavailable},
		{"internal -> 500", codes.Internal, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{err: status.Error(tt.code, "boom")}
			srv, _ := newTestGateway(t, svc)

			resp := doRequest(t, srv, http.MethodGet, "/v1/lists", "Bearer x", "")
			if resp.StatusCode != tt.wantHTTP {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantHTTP)
			}
			var body errorBody
			decodeJSON(t, resp, &body)
			if body.Code != tt.code.String() {
				t.Fatalf("error code = %q, want %q", body.Code, tt.code.String())
			}
			if body.Message != "boom" {
				t.Fatalf("error message = %q, want %q", body.Message, "boom")
			}
		})
	}
}

func TestGatewayRejectsInvalidJSONBody(t *testing.T) {
	svc := &fakeService{}
	srv, _ := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodPost, "/v1/lists", "Bearer x", `{not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid JSON", resp.StatusCode)
	}
}

func TestGatewayUnknownRouteReturns404(t *testing.T) {
	svc := &fakeService{}
	srv, _ := newTestGateway(t, svc)

	resp := doRequest(t, srv, http.MethodGet, "/v1/nope", "Bearer x", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for unknown route", resp.StatusCode)
	}
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON error = %v", err)
	}
}
