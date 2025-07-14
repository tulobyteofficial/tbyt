package modals

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB connection URI

// MongoDB client instance (singleton)
var client *mongo.Client

// ConnectDB establishes a connection to MongoDB
func ConnectDB() (*mongo.Database, error) {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		return nil, err
	}

	uri := fmt.Sprintf("mongodb://superadmin:%s@localhost:27017/admin", dbPassword)
	if client == nil { // Create client if not initialized
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return nil, err
		}
	}

	return client.Database("tulobyte_db"), nil
}
