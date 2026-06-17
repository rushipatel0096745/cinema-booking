package db

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var Pool *pgxpool.Pool

func Connect() *pgxpool.Pool {
	// Search upwards for .env file starting from the working directory
	dir, err := os.Getwd()
	loaded := false
	if err == nil {
		for {
			envPath := filepath.Join(dir, ".env")
			if _, err := os.Stat(envPath); err == nil {
				if err := godotenv.Load(envPath); err == nil {
					loaded = true
					break
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if !loaded {
		if err := godotenv.Load(); err != nil {
			log.Println("no .env file found, relying on environment variables")
		}
	}

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Fatalf("unable to parse DATABASE_URL: %v", err)
	}

	// Keep pool size modest since Neon's free/pro tiers cap concurrent connections
	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("unable to create connection pool: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("unable to ping database: %v", err)
	}

	log.Println("connected to Neon Postgres")
	// store pool in global variable
	Pool = pool
	return pool
}
