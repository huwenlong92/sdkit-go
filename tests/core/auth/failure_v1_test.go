package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/auth"
)

type v1Reader struct{}

func (v1Reader) ReadSession(context.Context, *http.Request, string) (any, bool, error) {
	return "user", true, nil
}
func TestV1SessionRefreshFailureRunsAllCleanup(t *testing.T) {
	cause := errors.New("refresh failed")
	cleanup := errors.New("cleanup failed")
	calls := 0
	a := auth.NewSessionAuthenticator(auth.SessionAuthenticator{Key: "user", Reader: v1Reader{}, Mapper: func(context.Context, any) (*auth.Identity, error) { return &auth.Identity{SubjectID: 1}, nil }, Refresher: func(context.Context, *auth.Identity, any) error { return cause }, Failures: []auth.IdentityFailureHook{
		func(context.Context, *http.Request, error) error { calls++; return cleanup },
		func(context.Context, *http.Request, error) error { calls++; return nil },
	}})
	identity, err := a.AuthenticateRequest(context.Background(), httptest.NewRequest("GET", "/", nil))
	if identity != nil || calls != 2 || !errors.Is(err, cause) || !errors.Is(err, cleanup) {
		t.Fatalf("identity=%v calls=%d err=%v", identity, calls, err)
	}
}

func TestV1ChainDoesNotHideFailedCleanupBehindAnonymousFallback(t *testing.T) {
	cleanup := errors.New("cleanup unavailable")
	called := false
	first := auth.RequestAuthenticatorFunc(func(context.Context, *http.Request) (*auth.Identity, error) {
		return nil, errors.Join(auth.ErrUnauthorized, auth.ErrSessionCleanup, cleanup)
	})
	fallback := auth.RequestAuthenticatorFunc(func(context.Context, *http.Request) (*auth.Identity, error) {
		called = true
		return &auth.Identity{Method: auth.MethodAnonymous}, nil
	})
	identity, err := auth.NewChainAuthenticator(first, fallback).AuthenticateRequest(context.Background(), httptest.NewRequest("GET", "/", nil))
	if called || identity != nil || !errors.Is(err, cleanup) {
		t.Fatalf("fallback=%v identity=%v err=%v", called, identity, err)
	}
}
