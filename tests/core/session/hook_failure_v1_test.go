package tests

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/huwenlong92/sdkit-go/core/gin/session"
)

func TestV1SetHookFailureRevokesIssuedCookie(t *testing.T) {
	middleware, err := session.Middleware(session.Config{Secret: "test-only-secret", Key: "v1_sid"})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(middleware)
	failure := errors.New("hook failed")
	router.GET("/login", func(c *gin.Context) {
		err := session.Set(c, "user", "alice", func(*gin.Context, session.Session, session.Options) error { return failure })
		if !errors.Is(err, failure) {
			t.Fatalf("Set=%v", err)
		}
		if value := session.Get(c, "user"); value != nil {
			t.Fatalf("failed login retained %v", value)
		}
		c.Status(http.StatusServiceUnavailable)
	})
	router.GET("/me", func(c *gin.Context) {
		if session.Get(c, "user") != nil {
			t.Error("failed login authenticated")
		}
		c.Status(http.StatusNoContent)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/login", nil))
	// Model browser replacement: only the final cookie of a given name survives.
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%v", cookies)
	}
	last := cookies[len(cookies)-1]
	if last.MaxAge >= 0 {
		t.Fatalf("final cookie MaxAge=%d", last.MaxAge)
	}
	req := httptest.NewRequest("GET", "/me", nil)
	req.AddCookie(last)
	router.ServeHTTP(httptest.NewRecorder(), req)
}

func TestV1MultipleSetHooksFailClosed(t *testing.T) {
	for _, failureIndex := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(failureIndex), func(t *testing.T) {
			middleware, err := session.Middleware(session.Config{Secret: "test-only-secret", Key: "multi_sid"})
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.Use(middleware)
			failure := errors.New("validation failed")
			calls := 0
			router.GET("/login", func(c *gin.Context) {
				hooks := []session.Hook{nil}
				for i := 0; i < 3; i++ {
					hooks = append(hooks, func(_ *gin.Context, s session.Session, _ session.Options) error {
						calls++
						if s.Get("user") != "alice" {
							t.Error("hook cannot see staged value")
						}
						if i == failureIndex {
							return failure
						}
						return nil
					})
				}
				if err := session.Set(c, "user", "alice", hooks...); !errors.Is(err, failure) {
					t.Fatalf("Set=%v", err)
				}
				if session.Get(c, "user") != nil {
					t.Error("failed credential retained")
				}
			})
			router.GET("/me", func(c *gin.Context) {
				if session.Get(c, "user") != nil {
					t.Error("failed login authenticated")
				}
			})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/login", nil))
			if calls != failureIndex+1 {
				t.Fatalf("hooks=%d, want fail-fast at %d", calls, failureIndex)
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
				t.Fatalf("cookies=%v", cookies)
			}
			req := httptest.NewRequest("GET", "/me", nil)
			req.AddCookie(cookies[0])
			router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestV1LogoutHookFailureKeepsSessionRevoked(t *testing.T) {
	for _, clearAll := range []bool{false, true} {
		t.Run(fmt.Sprintf("clear=%t", clearAll), func(t *testing.T) {
			middleware, err := session.Middleware(session.Config{Secret: "test-only-secret", Key: "logout_sid"})
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.Use(middleware)
			failure := errors.New("logout hook failed")
			calls := 0
			router.GET("/login", func(c *gin.Context) {
				if err := session.Set(c, "user", "alice"); err != nil {
					t.Fatal(err)
				}
			})
			router.GET("/logout", func(c *gin.Context) {
				hooks := []session.Hook{nil, func(_ *gin.Context, s session.Session, _ session.Options) error {
					calls++
					if s.Get("user") != nil {
						t.Error("hook sees revoked value")
					}
					return nil
				}, func(*gin.Context, session.Session, session.Options) error { calls++; return failure }, func(*gin.Context, session.Session, session.Options) error {
					t.Error("hook ran after failure")
					return nil
				}}
				var err error
				if clearAll {
					err = session.ClearWith(c, session.Options{MaxAge: -1}, hooks...)
				} else {
					err = session.DeleteWith(c, "user", session.Options{MaxAge: -1}, hooks...)
				}
				if !errors.Is(err, failure) {
					t.Fatalf("logout=%v", err)
				}
			})
			router.GET("/me", func(c *gin.Context) {
				if session.Get(c, "user") != nil {
					t.Error("logout failed to revoke credential")
				}
			})
			login := httptest.NewRecorder()
			router.ServeHTTP(login, httptest.NewRequest("GET", "/login", nil))
			req := httptest.NewRequest("GET", "/logout", nil)
			for _, cookie := range login.Result().Cookies() {
				req.AddCookie(cookie)
			}
			logout := httptest.NewRecorder()
			router.ServeHTTP(logout, req)
			if calls != 2 {
				t.Fatalf("hook calls=%d", calls)
			}
			cookies := logout.Result().Cookies()
			if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
				t.Fatalf("cookies=%v", cookies)
			}
			req = httptest.NewRequest("GET", "/me", nil)
			req.AddCookie(cookies[0])
			router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}
