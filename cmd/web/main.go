package main

import (
	"context"
	"log"
	"net/http"
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
	
	cartService := services.NewCartService()
	orderService := services.NewOrderService(productService, cartService)
	
	// Initialize handlers
	productHandler := handlers.NewProductHandler(productService)
	cartHandler := handlers.NewCartHandler(cartService, productService)
	orderHandler := handlers.NewOrderHandler(orderService, cartService, productService)
	
	// Setup router
	router := setupRouter(productHandler, cartHandler, orderHandler)
	
	// Start server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// Run server in goroutine
	go func() {
		log.Printf("🚀 Server starting on http://localhost:8080")
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

func setupRouter(productHandler *handlers.ProductHandler, cartHandler *handlers.CartHandler, orderHandler *handlers.OrderHandler) *gin.Engine {
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	
	// API Routes
	api := router.Group("/api")
	{
		// Product routes
		products := api.Group("/products")
		{
			products.GET("/", productHandler.GetAllProducts)
			products.GET("/:id", productHandler.GetProductByID)
			products.GET("/search", productHandler.SearchProducts)
			products.POST("/:id/purchase", productHandler.PurchaseProduct)
			products.PUT("/:id/stock", orderHandler.UpdateStock)
		}
		
		// Cart routes
		cart := api.Group("/cart")
		{
			cart.POST("/", cartHandler.CreateCart)
			cart.GET("/", cartHandler.GetCart)
			cart.POST("/items", cartHandler.AddToCart)
			cart.PUT("/items/:product_id", cartHandler.UpdateCartItem)
		}
		
		// Order routes
		orders := api.Group("/orders")
		{
			orders.POST("/", orderHandler.CreateOrder)
			orders.GET("/stats", orderHandler.GetStats)
		}
		
		// Flash sale
		api.POST("/flash-sale/:product_id/purchase", orderHandler.FlashSalePurchase)
		
		// Health check
		api.GET("/health", productHandler.HealthCheck)
	}
	
	// Debug endpoints in development
	if gin.Mode() != gin.ReleaseMode {
		router.GET("/debug/metrics", productHandler.Metrics)
	}
	
	return router
}