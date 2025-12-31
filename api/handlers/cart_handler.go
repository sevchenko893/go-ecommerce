package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-ecommerce/internal/models"
	"go-ecommerce/internal/services"
)

type CartHandler struct {
	cartService    *services.CartService
	productService *services.ProductService
}

func NewCartHandler(cartService *services.CartService, productService *services.ProductService) *CartHandler {
	return &CartHandler{
		cartService:    cartService,
		productService: productService,
	}
}

// POST /api/cart
// Create new cart for user
func (h *CartHandler) CreateCart(c *gin.Context) {
	userID := h.getCurrentUserID(c) // Pakai method receiver
	
	cart := h.cartService.CreateCart(userID)
	
	c.JSON(http.StatusCreated, gin.H{
		"cart_id": cart.ID,
		"user_id": cart.UserID,
	})
}

// GET /api/cart
// Get current user's cart
func (h *CartHandler) GetCart(c *gin.Context) {
	userID := h.getCurrentUserID(c) // Pakai method receiver
	
	cart, exists := h.cartService.GetCartByUserID(userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"data": cart,
	})
}

// POST /api/cart/items
// Add item to cart (RACE CONDITION TEST)
func (h *CartHandler) AddToCart(c *gin.Context) {
	userID := h.getCurrentUserID(c) // Pakai method receiver
	
	var req models.AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get user's cart
	cart, exists := h.cartService.GetCartByUserID(userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}
	
	// Get product info
	product, exists := h.productService.GetProductByID(req.ProductID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	
	// Check stock
	if product.Stock < req.Quantity {
		c.JSON(http.StatusConflict, gin.H{"error": "Insufficient stock"})
		return
	}
	
	// Choose which version to test
	mode := c.DefaultQuery("mode", "safe") // unsafe, safe, optimistic
	
	var updatedCart *models.Cart
	var err error
	
	switch mode {
	case "unsafe":
		// RACE CONDITION VERSION
		updatedCart, err = h.cartService.AddToCartNoLock(
			cart.ID, 
			req.ProductID, 
			req.Quantity, 
			product.Price, 
			product.Name,
		)
	case "optimistic":
		version, _ := strconv.Atoi(c.DefaultQuery("version", "1"))
		updatedCart, err = h.cartService.AddToCartOptimistic(
			cart.ID,
			req.ProductID,
			req.Quantity,
			product.Price,
			product.Name,
			version,
		)
	default: // "safe"
		updatedCart, err = h.cartService.AddToCartWithLock(
			cart.ID,
			req.ProductID,
			req.Quantity,
			product.Price,
			product.Name,
		)
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Item added to cart",
		"cart": updatedCart,
		"mode": mode,
	})
}

// PUT /api/cart/items/:product_id
// Update cart item quantity
func (h *CartHandler) UpdateCartItem(c *gin.Context) {
	userID := h.getCurrentUserID(c) // Pakai method receiver
	productID, err := strconv.Atoi(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	
	var req models.UpdateCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	cart, exists := h.cartService.GetCartByUserID(userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}
	
	// Choose version based on query param
	mode := c.DefaultQuery("mode", "safe")
	
	var updatedCart *models.Cart
	if mode == "unsafe" {
		updatedCart, err = h.cartService.UpdateCartItemQuantityRace(cart.ID, productID, req.Quantity)
	} else {
		updatedCart, err = h.cartService.UpdateCartItemQuantitySafe(cart.ID, productID, req.Quantity)
	}
	
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Cart updated",
		"cart": updatedCart,
		"mode": mode,
	})
}

// Helper function (jadikan method private dengan huruf kecil)
func (h *CartHandler) getCurrentUserID(c *gin.Context) int {
	// Method 1: Dari query parameter (untuk testing)
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		userID, _ := strconv.Atoi(userIDStr)
		return userID
	}
	
	// Method 2: Dari header (lebih realistic untuk production)
	if userIDStr := c.GetHeader("X-User-ID"); userIDStr != "" {
		userID, _ := strconv.Atoi(userIDStr)
		return userID
	}
	
	// Method 3: Dari JWT token (di real app)
	// claims, exists := c.Get("user_claims")
	// if exists {
	//     return claims.(*jwt.MapClaims)["user_id"].(int)
	// }
	
	// Default untuk testing
	return 1
}