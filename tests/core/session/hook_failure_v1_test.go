package tests

import (
	"errors"
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
