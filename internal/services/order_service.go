package services

import (
	"fmt"
	"sync"
	"time"

	"go-ecommerce/internal/models"
)

type OrderService struct {
	mu           sync.RWMutex
	orders       map[string]*models.Order // order_id -> order
	userOrders   map[int][]string         // user_id -> order_ids
	productService *ProductService
	cartService   *CartService
	
	// Statistics for monitoring
	stats struct {
		sync.RWMutex
		totalOrders   int64
		failedOrders  int64
		raceConditionDetected int64
	}
}

func NewOrderService(productService *ProductService, cartService *CartService) *OrderService {
	return &OrderService{
		orders:        make(map[string]*models.Order),
		userOrders:    make(map[int][]string),
		productService: productService,
		cartService:   cartService,
	}
}

// ============================================
// VERSION 1: DANGEROUS - NO INVENTORY LOCK
// ============================================
func (s *OrderService) CreateOrderNoLock(cartID string, userID int, address, paymentMethod string) (*models.Order, error) {
	// Get cart
	cart, exists := s.cartService.GetCart(cartID)
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Check cart belongs to user
	if cart.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	// Validate cart has items
	if len(cart.Items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	var total float64
	var orderItems []models.OrderItem

	// Process each item - RACE CONDITION DANGER ZONE!
	for _, item := range cart.Items {
		// Get current product info
		product, exists := s.productService.GetProductByID(item.ProductID)
		if !exists {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		// Check stock WITHOUT LOCK - RACE CONDITION!
		if product.Stock < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for product %s", product.Name)
		}

		// Simulate inventory check delay
		time.Sleep(time.Millisecond * 25)

		// Deduct stock WITHOUT LOCK - ANOTHER RACE CONDITION!
		product.Stock -= item.Quantity

		// Add to order items
		orderItems = append(orderItems, models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     product.Price,
			Name:      product.Name,
		})

		total += product.Price * float64(item.Quantity)
	}

	// Create order
	orderID := fmt.Sprintf("order_%d_%d", userID, time.Now().UnixNano())
	order := &models.Order{
		ID:           orderID,
		UserID:       userID,
		Items:        orderItems,
		Total:        total,
		Status:       models.OrderStatusPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	s.mu.Lock()
	s.orders[orderID] = order
	s.userOrders[userID] = append(s.userOrders[userID], orderID)
	s.stats.totalOrders++
	s.mu.Unlock()

	return order, nil
}

// ============================================
// VERSION 2: SAFE WITH DISTRIBUTED LOCK PATTERN
// ============================================
func (s *OrderService) CreateOrderSafe(cartID string, userID int, address, paymentMethod string) (*models.Order, error) {
	// Get cart
	cart, exists := s.cartService.GetCart(cartID)
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Check cart belongs to user
	if cart.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	// Validate cart has items
	if len(cart.Items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	// ===== CRITICAL SECTION START =====
	// Lock the entire order creation process
	// In real system, you might use distributed lock per product
	// lockKey := fmt.Sprintf("order_create_%d", userID)
	
	// Simulate acquiring distributed lock
	// time.Sleep(time.Millisecond * 10)
	
	defer func() {
		// Simulate releasing lock
		// In real system: releaseLock(lockKey)
	}()

	var total float64
	var orderItems []models.OrderItem
	var productsToUpdate []struct {
		productID int
		quantity  int
	}

	// Step 1: Validate inventory with locks
	for _, item := range cart.Items {
		product, exists := s.productService.GetProductByID(item.ProductID)
		if !exists {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		// Check stock with proper locking at service level
		// In real app, this would be a database transaction
		if product.Stock < item.Quantity {
			s.stats.Lock()
			s.stats.failedOrders++
			s.stats.Unlock()
			return nil, fmt.Errorf("insufficient stock for product %s", product.Name)
		}

		productsToUpdate = append(productsToUpdate, struct {
			productID int
			quantity  int
		}{
			productID: item.ProductID,
			quantity:  item.Quantity,
		})

		orderItems = append(orderItems, models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     product.Price,
			Name:      product.Name,
		})

		total += product.Price * float64(item.Quantity)
	}

	// Step 2: Update inventory (simulated atomic operation)
	for _, update := range productsToUpdate {
		success, err := s.productService.UpdateStock(update.productID, update.quantity)
		if !success || err != nil {
			// Rollback previous updates would go here
			return nil, fmt.Errorf("failed to update inventory for product %d", update.productID)
		}
	}

	// Step 3: Create order
	orderID := fmt.Sprintf("order_%d_%d", userID, time.Now().UnixNano())
	order := &models.Order{
		ID:           orderID,
		UserID:       userID,
		Items:        orderItems,
		Total:        total,
		Status:       models.OrderStatusPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	s.mu.Lock()
	s.orders[orderID] = order
	s.userOrders[userID] = append(s.userOrders[userID], orderID)
	s.stats.totalOrders++
	s.mu.Unlock()

	// Step 4: Clear cart (optional)
	// s.cartService.ClearCart(cartID)

	return order, nil
}

// ============================================
// VERSION 3: BATCH INVENTORY CHECK & UPDATE
// ============================================
func (s *OrderService) CreateOrderBatchCheck(cartID string, userID int) (*models.Order, error) {
	// This version tries to check all inventory at once
	// then update all at once to minimize race window

	cart, exists := s.cartService.GetCart(cartID)
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Build product quantity map
	productQuantities := make(map[int]int)
	var orderItems []models.OrderItem
	var total float64

	for _, item := range cart.Items {
		product, exists := s.productService.GetProductByID(item.ProductID)
		if !exists {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		productQuantities[item.ProductID] = item.Quantity
		
		orderItems = append(orderItems, models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     product.Price,
			Name:      product.Name,
		})

		total += product.Price * float64(item.Quantity)
	}

	// Try to reserve inventory for all products
	// This should be atomic in real database
	success := s.tryReserveInventory(productQuantities)
	if !success {
		s.stats.Lock()
		s.stats.raceConditionDetected++
		s.stats.Unlock()
		return nil, fmt.Errorf("inventory reservation failed - possible race condition")
	}

	// Create order
	orderID := fmt.Sprintf("order_%d_%d", userID, time.Now().UnixNano())
	order := &models.Order{
		ID:        orderID,
		UserID:    userID,
		Items:     orderItems,
		Total:     total,
		Status:    models.OrderStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.mu.Lock()
	s.orders[orderID] = order
	s.userOrders[userID] = append(s.userOrders[userID], orderID)
	s.stats.totalOrders++
	s.mu.Unlock()

	return order, nil
}

func (s *OrderService) tryReserveInventory(productQuantities map[int]int) bool {
	// Simulate atomic inventory reservation
	// In real app: database transaction with SELECT FOR UPDATE
	
	s.productService.mu.Lock()
	defer s.productService.mu.Unlock()

	// Check all products have enough stock
	for productID, quantity := range productQuantities {
		product, exists := s.productService.products[productID]
		if !exists || product.Stock < quantity {
			return false
		}
	}

	// Reserve inventory
	for productID, quantity := range productQuantities {
		s.productService.products[productID].Stock -= quantity
	}

	return true
}

// ============================================
// FLASH SALE RACE CONDITION SCENARIO
// ============================================
func (s *OrderService) FlashSalePurchase(productID, quantity, userID int) (*models.Order, error) {
	// Simulate flash sale scenario where thousands try to buy same product
	
	// Step 1: Check product exists and is in flash sale
	product, exists := s.productService.GetProductByID(productID)
	if !exists {
		return nil, fmt.Errorf("product not found")
	}

	// Step 2: Check stock - RACE CONDITION HOTSPOT!
	if product.Stock < quantity {
		return nil, fmt.Errorf("out of stock")
	}

	// Simulate user thinking time
	time.Sleep(time.Millisecond * time.Duration(50+(userID%100)))

	// Step 3: Update stock - ANOTHER RACE CONDITION!
	success, err := s.productService.UpdateStock(productID, quantity)
	if !success || err != nil {
		return nil, fmt.Errorf("purchase failed")
	}

	// Create order
	orderID := fmt.Sprintf("flash_%d_%d", userID, time.Now().UnixNano())
	order := &models.Order{
		ID:     orderID,
		UserID: userID,
		Items: []models.OrderItem{{
			ProductID: productID,
			Quantity:  quantity,
			Price:     product.Price,
			Name:      product.Name,
		}},
		Total:     product.Price * float64(quantity),
		Status:    models.OrderStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.mu.Lock()
	s.orders[orderID] = order
	s.userOrders[userID] = append(s.userOrders[userID], orderID)
	s.stats.totalOrders++
	s.mu.Unlock()

	return order, nil
}

// Statistics
func (s *OrderService) GetStats() map[string]int64 {
	s.stats.RLock()
	defer s.stats.RUnlock()

	return map[string]int64{
		"total_orders":          s.stats.totalOrders,
		"failed_orders":         s.stats.failedOrders,
		"race_conditions":       s.stats.raceConditionDetected,
	}
}