package handlers

import (
	"net/http"
	"strconv"
	"time"
	"runtime"

	"github.com/gin-gonic/gin"
	"go-ecommerce/internal/services"
)

type ProductHandler struct {
	productService *services.ProductService
}

func NewProductHandler(productService *services.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

// Get all products with pagination
func (h *ProductHandler) GetAllProducts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	products, total := h.productService.GetAllProducts(page, limit)

	totalPages := (total + limit - 1) / limit
	hasNext := page < totalPages
	hasPrev := page > 1

	c.JSON(http.StatusOK, gin.H{
		"data": products,
		"meta": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    hasNext,
			"has_prev":    hasPrev,
		},
	})
}

// Get product by ID
func (h *ProductHandler) GetProductByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	product, exists := h.productService.GetProductByID(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": product,
	})
}

// Search products
func (h *ProductHandler) SearchProducts(c *gin.Context) {
	query := c.Query("q")
	category := c.Query("category")
	minPrice, _ := strconv.ParseFloat(c.Query("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(c.Query("max_price"), 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	products, total := h.productService.SearchProducts(
		query, category, minPrice, maxPrice, page, limit,
	)

	totalPages := (total + limit - 1) / limit
	hasNext := page < totalPages
	hasPrev := page > 1

	c.JSON(http.StatusOK, gin.H{
		"data": products,
		"meta": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    hasNext,
			"has_prev":    hasPrev,
			"query":       query,
			"category":    category,
		},
	})
}

// Update product stock (simulate purchase)
func (h *ProductHandler) PurchaseProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var req struct {
		Quantity int `json:"quantity" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	success, err := h.productService.UpdateStock(id, req.Quantity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !success {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Insufficient stock or product not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Purchase successful",
		"product_id": id,
		"quantity":   req.Quantity,
	})
}

// Health check endpoint
func (h *ProductHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// Metrics endpoint
func (h *ProductHandler) Metrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"goroutines": runtime.NumGoroutine(),
		"timestamp":  time.Now().Unix(),
	})
}
