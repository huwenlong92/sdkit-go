package facade_test

import (
	"context"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/database"
	facade "github.com/huwenlong92/sdkit-go/core/database/facade"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func TestV1InjectedDatabaseIsBorrowed(t *testing.T) {
	db, err := database.New(context.Background(), database.Config{Driver: "sqlite", DSN: ":memory:"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := runtime.New()
	if err := app.Use(facade.Use(facade.WithDatabase(db))); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if db.Gorm == nil {
		t.Fatal("borrowed database was closed")
	}
	sqlDB, err := db.Gorm.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("borrowed database Ping=%v", err)
	}
}

func TestV1StoppingOneAppDoesNotCloseAnotherDatabase(t *testing.T) {
	first, second := runtime.New(), runtime.New()
	for _, app := range []*runtime.App{first, second} {
		if err := app.Use(facade.Use(facade.WithConfig(database.Config{Driver: "sqlite", DSN: ":memory:"}))); err != nil {
			t.Fatal(err)
		}
		if err := app.Run(); err != nil {
			t.Fatal(err)
		}
	}
	secondDB := database.From(second)
	if err := first.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if database.Default != secondDB || secondDB.Gorm == nil {
		t.Fatal("first app closed second database")
	}
	sqlDB, err := secondDB.Gorm.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := second.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	empty := runtime.New()
	if database.From(empty) != nil || database.GormFrom(empty) != nil {
		t.Fatal("empty app fell back to another default")
	}
}
