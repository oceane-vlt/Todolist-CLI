package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestCallTimeout(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "unset -> default", env: "", want: defaultCallTimeout},
		{name: "valid override", env: "3s", want: 3 * time.Second},
		{name: "invalid -> default", env: "not-a-duration", want: defaultCallTimeout},
		{name: "zero -> default", env: "0s", want: defaultCallTimeout},
		{name: "negative -> default", env: "-5s", want: defaultCallTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envCallTimeout, tc.env)
			if got := callTimeout(); got != tc.want {
				t.Fatalf("callTimeout() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTLSEnabled(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"1", true},
		{"true", true},
		{"0", false},
		{"false", false},
		{"garbage", false},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv(envTLSEnabled, tc.env)
			if got := tlsEnabled(); got != tc.want {
				t.Fatalf("tlsEnabled() with %q = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestTransportCredentialsSelection(t *testing.T) {
	t.Run("no TLS env -> insecure, no error", func(t *testing.T) {
		t.Setenv(envTLSEnabled, "")
		t.Setenv(envTLSCAFile, "")
		opt, err := transportCredentials()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opt == nil {
			t.Fatalf("expected a non-nil dial option")
		}
	})

	t.Run("TODO_TLS set -> system pool TLS, no error", func(t *testing.T) {
		t.Setenv(envTLSEnabled, "true")
		t.Setenv(envTLSCAFile, "")
		opt, err := transportCredentials()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opt == nil {
			t.Fatalf("expected a non-nil dial option")
		}
	})

	t.Run("missing CA file -> error", func(t *testing.T) {
		t.Setenv(envTLSCAFile, "/does/not/exist.pem")
		if _, err := transportCredentials(); err == nil {
			t.Fatalf("expected error for a missing CA file")
		}
	})
}

func TestTimeoutUnaryInterceptor(t *testing.T) {
	t.Run("adds a deadline when none is set", func(t *testing.T) {
		t.Setenv(envCallTimeout, "5s")
		var sawDeadline bool
		invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			_, sawDeadline = ctx.Deadline()
			return nil
		}
		if err := timeoutUnaryInterceptor(context.Background(), "/svc/M", nil, nil, nil, invoker); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sawDeadline {
			t.Fatalf("expected the interceptor to set a deadline on the context")
		}
	})

	t.Run("preserves a caller-supplied deadline", func(t *testing.T) {
		caller, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		wantDeadline, _ := caller.Deadline()

		var gotDeadline time.Time
		invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			gotDeadline, _ = ctx.Deadline()
			return nil
		}
		if err := timeoutUnaryInterceptor(caller, "/svc/M", nil, nil, nil, invoker); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !gotDeadline.Equal(wantDeadline) {
			t.Fatalf("expected caller deadline %v preserved, got %v", wantDeadline, gotDeadline)
		}
	})

	t.Run("propagates the invoker error", func(t *testing.T) {
		wantErr := errors.New("boom")
		invoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			return wantErr
		}
		if err := timeoutUnaryInterceptor(context.Background(), "/svc/M", nil, nil, nil, invoker); !errors.Is(err, wantErr) {
			t.Fatalf("expected invoker error to propagate, got %v", err)
		}
	})
}
