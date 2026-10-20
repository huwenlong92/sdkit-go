package redis_test

import (
	"context"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/redis"
	"github.com/huwenlong92/sdkit-go/core/runtime"
	goredis "github.com/redis/go-redis/v9"
)

func TestClientFromDoesNotFallBackAcrossApps(t *testing.T) {
	owner, isolated := runtime.New(), runtime.New()
	client := goredis.NewClient(&goredis.Options{Addr: "localhost:1"})
	t.Cleanup(func() {
		_ = owner.Stop(context.Background())
		_ = isolated.Stop(context.Background())
		_ = client.Close()
	})
	if err := redis.Bind(owner, &redis.RuntimeClient{Rdb: client}); err != nil {
		t.Fatal(err)
	}
	if redis.ClientFrom(nil) != client || redis.ClientFrom(owner) != client {
		t.Fatal("default and owner must resolve the injected client")
	}
	if redis.ClientFrom(isolated) != nil {
		t.Fatal("an explicit App without Redis must not borrow another App's default")
	}
	if err := isolated.Container().Bind(redis.KeyRedis, &redis.RuntimeClient{}); err != nil {
		t.Fatal(err)
	}
	if redis.ClientFrom(isolated) != nil {
		t.Fatal("an explicit empty client must not fall back to the process default")
	}
	if err := owner.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if redis.ClientFrom(nil) != nil {
		t.Fatal("default remains published after owner shutdown")
	}
	// Closing the borrowed client here must still be the caller's responsibility.
	if err := client.Close(); err != nil {
		t.Fatalf("owner closed an injected client: %v", err)
	}
}
