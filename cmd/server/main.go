package main

import (
	"cinemabooking/internal/db"

	"github.com/gin-gonic/gin"
)

func main() {
	db.Connect()
	defer db.Pool.Close()

	r := gin.Default()

	// Add routes here
	r.Run()
}
