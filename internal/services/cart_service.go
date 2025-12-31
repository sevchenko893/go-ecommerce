package services

import (
	"fmt"
	"sync"
	"time"

	"go-ecommerce/internal/models"
)

type CartService struct {
	mu      sync.RWMutex
	carts   map[string]*models.Cart // cart_id -> cart
	userCarts map[int]string       // user_id -> cart_id
}

func NewCartService() *CartService {
	return &CartService{
		carts:     make(map[string]*models.Cart),
		userCarts: make(map[int]string),
	}
}

// ============================================
// VERSION 1: TANPA LOCK - RACE CONDITION BAKAL TERJADI!
// ============================================
func (s *CartService) AddToCartNoLock(cartID string, productID, quantity int, productPrice float64, productName string) (*models.Cart, error) {
	cart, exists := s.carts[cartID]
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Simulate network/database delay
	time.Sleep(time.Millisecond * 20)

	// Check if product already in cart
	for i, item := range cart.Items {
		if item.ProductID == productID {
			// VULNERABLE TO RACE CONDITION!
			cart.Items[i].Quantity += quantity
			cart.Items[i].Price = productPrice
			cart.UpdatedAt = time.Now()
			return cart, nil
		}
	}

	// Add new item
	cart.Items = append(cart.Items, models.CartItem{
		ProductID: productID,
		Quantity:  quantity,
		Price:     productPrice,
		Name:      productName,
		AddedAt:   time.Now(),
	})
	cart.UpdatedAt = time.Now()

	return cart, nil
}

// ============================================
// VERSION 2: DENGAN MUTEX LOCK - AMAN
// ============================================
func (s *CartService) AddToCartWithLock(cartID string, productID, quantity int, productPrice float64, productName string) (*models.Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, exists := s.carts[cartID]
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Simulate network/database delay
	time.Sleep(time.Millisecond * 20)

	// Check if product already in cart
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items[i].Quantity += quantity
			cart.Items[i].Price = productPrice
			cart.UpdatedAt = time.Now()
			return cart, nil
		}
	}

	// Add new item
	cart.Items = append(cart.Items, models.CartItem{
		ProductID: productID,
		Quantity:  quantity,
		Price:     productPrice,
		Name:      productName,
		AddedAt:   time.Now(),
	})
	cart.UpdatedAt = time.Now()

	return cart, nil
}

// ============================================
// VERSION 3: OPTIMISTIC LOCKING - DATABASE STYLE
// ============================================
func (s *CartService) AddToCartOptimistic(cartID string, productID, quantity int, productPrice float64, productName string, version int) (*models.Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, exists := s.carts[cartID]
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Check version for optimistic locking
	if cart.Version != version {
		return nil, fmt.Errorf("cart was modified by another request")
	}

	// Simulate network/database delay
	time.Sleep(time.Millisecond * 20)

	// Check if product already in cart
	found := false
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items[i].Quantity += quantity
			cart.Items[i].Price = productPrice
			found = true
			break
		}
	}

	if !found {
		cart.Items = append(cart.Items, models.CartItem{
			ProductID: productID,
			Quantity:  quantity,
			Price:     productPrice,
			Name:      productName,
			AddedAt:   time.Now(),
		})
	}

	cart.UpdatedAt = time.Now()
	cart.Version++ // Increment version

	return cart, nil
}

// ============================================
// RACE CONDITION DEMO: Update Quantity
// ============================================
func (s *CartService) UpdateCartItemQuantityRace(cartID string, productID, quantity int) (*models.Cart, error) {
	// NO LOCK - This will cause race condition!
	cart, exists := s.carts[cartID]
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Simulate processing delay (makes race condition more likely)
	time.Sleep(time.Millisecond * 30)

	for i, item := range cart.Items {
		if item.ProductID == productID {
			// RACE CONDITION HERE!
			// If two requests update at same time, one will be lost
			cart.Items[i].Quantity = quantity
			cart.UpdatedAt = time.Now()
			return cart, nil
		}
	}

	return nil, fmt.Errorf("product not found in cart")
}

// ============================================
// SAFE VERSION: With Lock
// ============================================
func (s *CartService) UpdateCartItemQuantitySafe(cartID string, productID, quantity int) (*models.Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, exists := s.carts[cartID]
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Simulate processing delay
	time.Sleep(time.Millisecond * 30)

	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items[i].Quantity = quantity
			cart.UpdatedAt = time.Now()
			return cart, nil
		}
	}

	return nil, fmt.Errorf("product not found in cart")
}

// Helper methods
func (s *CartService) CreateCart(userID int) *models.Cart {
	s.mu.Lock()
	defer s.mu.Unlock()

	cartID := fmt.Sprintf("cart_%d_%d", userID, time.Now().UnixNano())
	cart := &models.Cart{
		ID:        cartID,
		UserID:    userID,
		Items:     []models.CartItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	s.carts[cartID] = cart
	s.userCarts[userID] = cartID

	return cart
}

func (s *CartService) GetCart(cartID string) (*models.Cart, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cart, exists := s.carts[cartID]
	return cart, exists
}

func (s *CartService) GetCartByUserID(userID int) (*models.Cart, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cartID, exists := s.userCarts[userID]
	if !exists {
		return nil, false
	}

	cart, exists := s.carts[cartID]
	return cart, exists
}