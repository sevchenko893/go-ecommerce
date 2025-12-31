package models

import "time"

type Cart struct {
	ID        string         `json:"id"`
	UserID    int            `json:"user_id"`
	Items     []CartItem     `json:"items"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Version   int            `json:"-"` // Optimistic locking
}

type CartItem struct {
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Name      string  `json:"name"`
	AddedAt   time.Time `json:"added_at"`
}

type AddToCartRequest struct {
	ProductID int `json:"product_id" binding:"required"`
	Quantity  int `json:"quantity" binding:"required,gt=0"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity" binding:"required,gt=0"`
}