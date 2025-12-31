package services

import (
	"sync"
	"time"
	
	"go-ecommerce/internal/models"
)

type ProductService struct {
	mu       sync.RWMutex
	products map[int]*models.Product
	nextID   int
}

func NewProductService() *ProductService {
	// Initialize with some sample products
	return &ProductService{
		products: make(map[int]*models.Product),
		nextID:   1,
	}
}

func (s *ProductService) InitSampleData() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add 1000 sample products for load testing
	for i := 1; i <= 1000; i++ {
		s.products[i] = &models.Product{
			ID:          i,
			Name:        "Product " + string(rune('A' + (i%26))),
			Description: "Description for product " + string(rune('A' + (i%26))),
			Price:       float64((i%1000) + 1),
			Stock:       (i % 100) + 1,
			Category:    []string{"Electronics", "Clothing", "Books", "Home"}[i%4],
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}
	s.nextID = 1001
}

// Get all products (with pagination) - READ HEAVY
func (s *ProductService) GetAllProducts(page, limit int) ([]models.Product, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Simulate database query delay
	time.Sleep(time.Millisecond * 10)

	total := len(s.products)
	if total == 0 {
		return []models.Product{}, 0
	}

	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []models.Product{}, total
	}
	if end > total {
		end = total
	}

	products := make([]models.Product, 0, end-start)
	for i := start; i < end; i++ {
		if product, exists := s.products[i+1]; exists {
			products = append(products, *product)
		}
	}

	return products, total
}

// Get product by ID - READ HEAVY
func (s *ProductService) GetProductByID(id int) (*models.Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Simulate database query delay
	time.Sleep(time.Millisecond * 5)

	product, exists := s.products[id]
	if !exists {
		return nil, false
	}

	return product, true
}

// Search products - READ HEAVY with filtering
func (s *ProductService) SearchProducts(query string, category string, minPrice, maxPrice float64, page, limit int) ([]models.Product, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Simulate complex search delay
	time.Sleep(time.Millisecond * 20)

	var results []models.Product
	for _, product := range s.products {
		// Simple search logic
		matchesQuery := query == "" || 
			contains(product.Name, query) || 
			contains(product.Description, query)
		matchesCategory := category == "" || product.Category == category
		matchesPrice := (minPrice == 0 || product.Price >= minPrice) &&
			(maxPrice == 0 || product.Price <= maxPrice)

		if matchesQuery && matchesCategory && matchesPrice {
			results = append(results, *product)
		}
	}

	// Pagination
	total := len(results)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []models.Product{}, total
	}
	if end > total {
		end = total
	}

	return results[start:end], total
}

// Update stock - WRITE with potential RACE CONDITION
func (s *ProductService) UpdateStock(productID, quantity int) (bool, error) {
	// VERSION 1: Tanpa lock - ini akan menyebabkan race condition!
	// product, exists := s.products[productID]
	// if !exists {
	//     return false, nil
	// }
	
	// // Simulate processing delay
	// time.Sleep(time.Millisecond * 15)
	
	// if product.Stock < quantity {
	//     return false, nil
	// }
	
	// product.Stock -= quantity
	// return true, nil

	// VERSION 2: Dengan lock - ini aman
	s.mu.Lock()
	defer s.mu.Unlock()

	product, exists := s.products[productID]
	if !exists {
		return false, nil
	}

	// Simulate processing delay
	time.Sleep(time.Millisecond * 15)

	if product.Stock < quantity {
		return false, nil
	}

	product.Stock -= quantity
	product.UpdatedAt = time.Now()
	return true, nil
}

// Helper function for search
func contains(s, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}