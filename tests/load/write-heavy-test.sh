#!/bin/bash

echo "========================================="
echo "🔥 WRITE-HEAVY RACE CONDITION LOAD TEST"
echo "========================================="

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if vegeta is installed
if ! command -v vegeta &> /dev/null; then
    echo -e "${RED}❌ Vegeta not installed!${NC}"
    echo "Installing vegeta..."
    go install github.com/tsenart/vegeta@latest
    export PATH=$PATH:$(go env GOPATH)/bin
fi

# Check if server is running
echo -e "${YELLOW}Checking if server is running...${NC}"
if ! curl -s http://localhost:8080/api/health > /dev/null; then
    echo -e "${RED}❌ Server not running on port 8080${NC}"
    echo "Start the server first: go run cmd/web/main.go"
    exit 1
fi

echo -e "${GREEN}✅ Server is running${NC}"
echo ""

# Create initial cart for testing
echo -e "${YELLOW}Creating test cart...${NC}"
CART_RESPONSE=$(curl -s -X POST "http://localhost:8080/api/cart?user_id=999")
CART_ID=$(echo $CART_RESPONSE | grep -o '"cart_id":"[^"]*' | cut -d'"' -f4)
echo "Cart ID: $CART_ID"
echo ""

# Reset product stock to 100 for testing
echo -e "${YELLOW}Resetting product stock...${NC}"
curl -X PUT "http://localhost:8080/api/products/1/stock" \
  -H "Content-Type: application/json" \
  -d '{"stock": 100}' -s > /dev/null
echo "Product 1 stock reset to 100"
echo ""

# ==========================================
# TEST 1: ADD TO CART RACE CONDITION
# ==========================================
echo "========================================="
echo "1. ADD TO CART RACE CONDITION TEST"
echo "========================================="
echo -e "${YELLOW}100 users adding same product to cart at same time${NC}"
echo "Mode: unsafe (no locking)"

cat > cart_targets.txt << EOF
POST http://localhost:8080/api/cart/items?user_id=999&mode=unsafe
Content-Type: application/json
{"product_id": 1, "quantity": 1}

POST http://localhost:8080/api/cart/items?user_id=999&mode=unsafe
Content-Type: application/json
{"product_id": 1, "quantity": 1}
EOF

vegeta attack -targets=cart_targets.txt \
  -rate=100 \
  -duration=2s \
  -max-workers=200 \
  -timeout=5s | vegeta report

echo ""
echo -e "${YELLOW}Checking cart quantity after race...${NC}"
curl -s "http://localhost:8080/api/cart?user_id=999" | jq '.data.items[] | select(.product_id==1) | .quantity'

echo ""
# ==========================================
# TEST 2: SAFE ADD TO CART
# ==========================================
echo "========================================="
echo "2. SAFE ADD TO CART TEST"
echo "========================================="
echo -e "${YELLOW}Same test but with locking${NC}"
echo "Mode: safe (with mutex lock)"

# Clear cart first
echo -e "${YELLOW}Clearing cart...${NC}"
NEW_CART_RESPONSE=$(curl -s -X POST "http://localhost:8080/api/cart?user_id=998")
NEW_CART_ID=$(echo $NEW_CART_RESPONSE | grep -o '"cart_id":"[^"]*' | cut -d'"' -f4)

cat > cart_safe_targets.txt << EOF
POST http://localhost:8080/api/cart/items?user_id=998&mode=safe
Content-Type: application/json
{"product_id": 1, "quantity": 1}
EOF

vegeta attack -targets=cart_safe_targets.txt \
  -rate=100 \
  -duration=2s \
  -max-workers=200 \
  -timeout=5s | vegeta report

echo ""
echo -e "${YELLOW}Checking cart quantity (should be exactly 100)...${NC}"
curl -s "http://localhost:8080/api/cart?user_id=998" | jq '.data.items[] | select(.product_id==1) | .quantity'

echo ""
# ==========================================
# TEST 3: FLASH SALE RACE CONDITION
# ==========================================
echo "========================================="
echo "3. FLASH SALE EXTREME RACE CONDITION"
echo "========================================="
echo -e "${RED}⚠️  WARNING: This will test inventory oversell!${NC}"
echo -e "${YELLOW}1000 users trying to buy 100 available products${NC}"

# Reset stock to 100
curl -X PUT "http://localhost:8080/api/products/2/stock" \
  -H "Content-Type: application/json" \
  -d '{"stock": 100}' -s > /dev/null

cat > flash_sale_targets.txt << EOF
POST http://localhost:8080/api/flash-sale/2/purchase?user_id=1001&quantity=1
POST http://localhost:8080/api/flash-sale/2/purchase?user_id=1002&quantity=1
POST http://localhost:8080/api/flash-sale/2/purchase?user_id=1003&quantity=1
EOF

echo -e "${YELLOW}Starting flash sale attack...${NC}"
vegeta attack -targets=flash_sale_targets.txt \
  -rate=200 \
  -duration=5s \
  -max-workers=500 \
  -timeout=10s | vegeta report > flash_sale_report.txt

echo ""
echo -e "${YELLOW}Flash Sale Results:${NC}"
echo "Total requests made: 1000"
echo -e "${YELLOW}Checking product 2 stock (should be 0, but might be negative!):${NC}"
curl -s "http://localhost:8080/api/products/2" | jq '.data.stock'

echo -e "${YELLOW}Order stats:${NC}"
curl -s "http://localhost:8080/api/orders/stats" | jq

echo ""
# ==========================================
# TEST 4: ORDER CREATION RACE
# ==========================================
echo "========================================="
echo "4. ORDER CREATION RACE CONDITION"
echo "========================================="

# Create multiple carts for testing
echo -e "${YELLOW}Creating test carts...${NC}"
declare -a cart_ids
for i in {1..10}; do
    response=$(curl -s -X POST "http://localhost:8080/api/cart?user_id=$((2000+i))")
    cart_id=$(echo $response | grep -o '"cart_id":"[^"]*' | cut -d'"' -f4)
    cart_ids+=($cart_id)
    
    # Add item to each cart
    curl -X POST "http://localhost:8080/api/cart/items?user_id=$((2000+i))&mode=safe" \
      -H "Content-Type: application/json" \
      -d '{"product_id": 3, "quantity": 2}' -s > /dev/null
done

echo "Created ${#cart_ids[@]} carts with items"

# Test unsafe order creation
cat > order_targets.txt << EOF
POST http://localhost:8080/api/orders?user_id=2001&mode=unsafe
Content-Type: application/json
{"cart_id": "${cart_ids[0]}", "address": "Test Address", "payment_method": "cash"}

POST http://localhost:8080/api/orders?user_id=2002&mode=unsafe
Content-Type: application/json
{"cart_id": "${cart_ids[1]}", "address": "Test Address", "payment_method": "cash"}
EOF

echo -e "${YELLOW}Testing order creation race condition...${NC}"
vegeta attack -targets=order_targets.txt \
  -rate=50 \
  -duration=3s \
  -max-workers=100 \
  -timeout=10s | vegeta report

echo ""
echo -e "${YELLOW}Checking product 3 stock after race...${NC}"
INITIAL_STOCK=$(curl -s "http://localhost:8080/api/products/3" | jq '.data.stock')
echo "Initial stock: $INITIAL_STOCK"

# Count successful orders
echo -e "${YELLOW}Counting successful orders...${NC}"
STATS=$(curl -s "http://localhost:8080/api/orders/stats")
TOTAL_ORDERS=$(echo $STATS | jq '.stats.total_orders')
FAILED_ORDERS=$(echo $STATS | jq '.stats.failed_orders')
RACE_CONDITIONS=$(echo $STATS | jq '.stats.race_conditions')

echo "Total orders: $TOTAL_ORDERS"
echo "Failed orders: $FAILED_ORDERS"
echo "Race conditions detected: $RACE_CONDITIONS"

echo ""
# ==========================================
# TEST 5: MIXED WORKLOAD
# ==========================================
echo "========================================="
echo "5. MIXED READ/WRITE WORKLOAD"
echo "========================================="

cat > mixed_targets.txt << EOF
GET http://localhost:8080/api/products
GET http://localhost:8080/api/products/1
GET http://localhost:8080/api/products/search?q=product
POST http://localhost:8080/api/cart/items?user_id=3001&mode=unsafe
Content-Type: application/json
{"product_id": 4, "quantity": 1}
POST http://localhost:8080/api/cart/items?user_id=3002&mode=safe
Content-Type: application/json
{"product_id": 4, "quantity": 1}
POST http://localhost:8080/api/products/5/purchase
Content-Type: application/json
{"quantity": 1}
EOF

echo -e "${YELLOW}Running mixed workload (60 seconds)...${NC}"
vegeta attack -targets=mixed_targets.txt \
  -rate=100 \
  -duration=60s \
  -max-workers=300 \
  -timeout=10s | vegeta report

echo ""
# ==========================================
# FINAL RESULTS
# ==========================================
echo "========================================="
echo "🎯 TEST RESULTS SUMMARY"
echo "========================================="

echo -e "${YELLOW}Final Stock Status:${NC}"
for i in {1..5}; do
    stock=$(curl -s "http://localhost:8080/api/products/$i" | jq '.data.stock')
    echo "Product $i stock: $stock"
    if [[ $stock -lt 0 ]]; then
        echo -e "${RED}⚠️  RACE CONDITION DETECTED! Negative stock!${NC}"
    fi
done

echo ""
echo -e "${YELLOW}Order Statistics:${NC}"
curl -s "http://localhost:8080/api/orders/stats" | jq '.stats'

echo ""
echo -e "${GREEN}✅ Load test completed!${NC}"
echo "Check for negative stock values to identify race conditions"

# Cleanup
rm -f cart_targets.txt cart_safe_targets.txt flash_sale_targets.txt order_targets.txt mixed_targets.txt