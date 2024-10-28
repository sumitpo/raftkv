package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gookit/slog"
)

func requestVote(c *gin.Context) {
}

func main() {
	slog.Configure(func(logger *slog.SugaredLogger) {
		f := logger.Formatter.(*slog.TextFormatter)
		f.EnableColor = true
	})

	// Create a new Gin router
	router := gin.Default()

	router.POST("/requestVote", requestVote)

	// Define a simple GET route
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Define another GET route with a path parameter
	router.GET("/hello/:name", func(c *gin.Context) {
		name := c.Param("name")
		c.JSON(http.StatusOK, gin.H{"message": "Hello " + name})
	})

	// Start the server on port 8080
	router.Run(":8081")
}
