package main

import (
	"context"   // <-- tambahkan
	"net/http"  // <-- tambahkan
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go-ecommerce/api/handlers"
	"go-ecommerce/internal/services"
)

func main() {
	// Initialize services
	productService := services.NewProductService()
	productService.InitSampleData()

	// Initialize handlers
	productHandler := handlers.NewProductHandler(productService)

	// Setup Gin router
	router := setupRouter(productHandler)

	// Start server in goroutine
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 Server starting on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	
	log.Println("✅ Server shutdown complete")
}

func setupRouter(productHandler *handlers.ProductHandler) *gin.Engine {
	// Set Gin mode based on environment
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	
	// Global middlewares
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	
	// Routes
	api := router.Group("/api")
	{
		// Product routes
		products := api.Group("/products")
		{
			products.GET("/", productHandler.GetAllProducts)
			products.GET("/:id", productHandler.GetProductByID)
			products.GET("/search", productHandler.SearchProducts)
			products.POST("/:id/purchase", productHandler.PurchaseProduct)
		}
		
		// Health check
		api.GET("/health", productHandler.HealthCheck)
	}
	
	// Debug endpoints (only in development)
	if gin.Mode() != gin.ReleaseMode {
		router.GET("/debug/metrics", productHandler.Metrics)
	}
	
	return router
}