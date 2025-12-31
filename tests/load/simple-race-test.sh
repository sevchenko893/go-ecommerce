#!/bin/bash

echo "🔥 SIMPLE RACE CONDITION TEST"
echo "=============================="

# Install tools if needed
if ! command -v vegeta &> /dev/null; then
    echo "Installing vegeta..."
    go install github.com/tsenart/vegeta@latest
    export PATH=$PATH:$(go env GOPATH)/bin
fi

if ! command -v jq &> /dev/null; then
    echo "Please install jq first"
    exit 1
fi

# Check server
if ! curl -s http://localhost:8080/api/health > /dev/null; then
    echo "Start server first: go run cmd/web/main.go"
    exit 1
fi

echo "✅ Server is running"
echo ""

# TEST 1: Purchase endpoint race
echo "1. Testing purchase endpoint race condition..."
echo "Resetting product 1 stock to 50..."
curl -X PUT "http://localhost:8080/api/products/1/stock" \
  -H "Content-Type: application/json" \
  -d '{"stock": 50}' -s > /dev/null

echo "Creating vegeta target file..."
cat > test_purchase.txt << EOF
POST http://localhost:8080/api/products/1/purchase
Content-Type: application/json

{"quantity": 1}
EOF

echo "Sending 100 concurrent purchase requests..."
vegeta attack -targets=test_purchase.txt \
  -rate=100 \
  -duration=1s \
  -workers=100 \
  -timeout=5s | vegeta report

echo ""
echo "Checking final stock..."
FINAL_STOCK=$(curl -s "http://localhost:8080/api/products/1" | jq '.data.stock')
echo "Final stock: $FINAL_STOCK"

if [[ $FINAL_STOCK -lt 0 ]]; then
    echo "❌ RACE CONDITION: Stock negative!"
elif [[ $FINAL_STOCK -gt 0 ]]; then
    echo "⚠️  Some purchases failed (expected 0, got $FINAL_STOCK)"
else
    echo "✅ All purchases processed correctly"
fi

# TEST 2: Add to cart
echo ""
echo "2. Testing add to cart..."
echo "Creating cart for user 777..."
CART_RESPONSE=$(curl -s -X POST "http://localhost:8080/api/cart?user_id=777")
CART_ID=$(echo $CART_RESPONSE | jq -r '.cart_id')
echo "Cart ID: $CART_ID"

cat > test_cart.txt << EOF
POST http://localhost:8080/api/cart/items?user_id=777&mode=unsafe
Content-Type: application/json

{"product_id": 2, "quantity": 1}
EOF

echo "Sending 50 concurrent add-to-cart requests..."
vegeta attack -targets=test_cart.txt \
  -rate=50 \
  -duration=1s \
  -workers=100 \
  -timeout=5s | vegeta report

echo ""
echo "Checking cart..."
curl -s "http://localhost:8080/api/cart?user_id=777" | jq '.data'

# Cleanup
rm -f test_purchase.txt test_cart.txt

echo ""
echo "✅ Test completed!"