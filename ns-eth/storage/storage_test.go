package storage

import (
	"os"
	"testing"
)

var (
	config Config
)

func TestMain(m *testing.M) {
	config = Config{
		Address: "localhost:4444", // os.Getenv("MONGO_DB_ADDRESS"),
		Username: "", // os.Getenv("MONGO_DB_USER"),
		Password: "", // os.Getenv("MONGO_DB_PASSWORD"),
		Database: "ethns.main", // os.Getenv("MONGO_DB_DATABASE"),
	}

	os.Exit(m.Run())
}

func TestNewStorage(test *testing.T) {
	storage, err := NewStorage(config)

	if err != nil {
		test.Fatalf("failed to build a new Store instance: %v", err)
	}

	storage.Close()
}