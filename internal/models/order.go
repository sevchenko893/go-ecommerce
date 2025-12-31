package models

import "time"

type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"
	OrderStatusProcessing OrderStatus = "processing"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

type Order struct {
	ID         string      `json:"id"`
	UserID     int         `json:"user_id"`
	Items      []OrderItem `json:"items"`
	Total      float64     `json:"total"`
	Status     OrderStatus `json:"status"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Name      string  `json:"name"`
}

type CreateOrderRequest struct {
	CartID    string `json:"cart_id" binding:"required"`
	Address   string `json:"address" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}