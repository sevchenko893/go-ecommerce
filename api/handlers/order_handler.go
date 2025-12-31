package handlers

import (
	"net/http"
	"strconv"
	// "time"

	"github.com/gin-gonic/gin"
	"go-ecommerce/internal/models"
	"go-ecommerce/internal/services"
)

type OrderHandler struct {
	orderService   *services.OrderService
	cartService    *services.CartService
	productService *services.ProductService
}

func NewOrderHandler(orderService *services.OrderService, cartService *services.CartService, productService *services.ProductService) *OrderHandler {
	return &OrderHandler{
		orderService:   orderService,
		cartService:    cartService,
		productService: productService,
	}
}

// POST /api/orders
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID := h.getCurrentUserID(c) // Pakai method receiver
	
	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	mode := c.DefaultQuery("mode", "safe")
	
	var order *models.Order
	var err error
	
	switch mode {
	case "unsafe":
		order, err = h.orderService.CreateOrderNoLock(
			req.CartID,
			userID,
			req.Address,
			req.PaymentMethod,
		)
	case "batch":
		order, err = h.orderService.CreateOrderBatchCheck(req.CartID, userID)
	default:
		order, err = h.orderService.CreateOrderSafe(
			req.CartID,
			userID,
			req.Address,
			req.PaymentMethod,
		)
	}
	
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"order": order,
		"mode":  mode,
	})
}

// POST /api/flash-sale/:product_id/purchase
func (h *OrderHandler) FlashSalePurchase(c *gin.Context) {
	userID := h.getCurrentUserID(c) // Pakai method receiver
	productID, err := strconv.Atoi(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	
	quantity, _ := strconv.Atoi(c.DefaultQuery("quantity", "1"))
	
	order, err := h.orderService.FlashSalePurchase(productID, quantity, userID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "Flash sale purchase successful!",
		"order":   order,
	})
}

// PUT /api/products/:id/stock
func (h *OrderHandler) UpdateStock(c *gin.Context) {
	productID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	
	var req struct {
		Stock int `json:"stock" binding:"required,gte=0"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	product, exists := h.productService.GetProductByID(productID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	
	success := h.productService.UpdateStockDirect(productID, req.Stock)
	if !success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stock"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":    "Stock updated",
		"product_id": productID,
		"old_stock":  product.Stock,
		"new_stock":  req.Stock,
	})
}

// GET /api/orders/stats
func (h *OrderHandler) GetStats(c *gin.Context) {
	stats := h.orderService.GetStats()
	
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// Helper function (jadikan method private)
func (h *OrderHandler) getCurrentUserID(c *gin.Context) int {
	// Method 1: Dari query parameter
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		userID, _ := strconv.Atoi(userIDStr)
		return userID
	}
	
	// Method 2: Dari header
	if userIDStr := c.GetHeader("X-User-ID"); userIDStr != "" {
		userID, _ := strconv.Atoi(userIDStr)
		return userID
	}
	
	// Default untuk testing
	return 1
}