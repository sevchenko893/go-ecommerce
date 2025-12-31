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

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}❌ jq not installed!${NC}"
    echo "Install jq: brew install jq (Mac) or apt-get install jq (Linux)"
    exit 1
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
CART_ID=$(echo $CART_RESPONSE | jq -r '.cart_id')
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

# FIX: Buat file vegeta targets dengan format yang benar
cat > cart_targets.txt << EOF
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
NEW_CART_ID=$(echo $NEW_CART_RESPONSE | jq -r '.cart_id')
echo "New Cart ID: $NEW_CART_ID"

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

# FIX: Flash sale tanpa body karena menggunakan query params
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
cat flash_sale_report.txt

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
for i in {1..5}; do
    USER_ID=$((2000+i))
    response=$(curl -s -X POST "http://localhost:8080/api/cart?user_id=$USER_ID")
    cart_id=$(echo $response | jq -r '.cart_id')
    cart_ids+=($cart_id)
    
    echo "Created cart for user $USER_ID: $cart_id"
    
    # Add item to each cart
    curl -X POST "http://localhost:8080/api/cart/items?user_id=$USER_ID&mode=safe" \
      -H "Content-Type: application/json" \
      -d '{"product_id": 3, "quantity": 2}' -s > /dev/null
done

echo "Created ${#cart_ids[@]} carts with items"

# FIX: Order creation dengan format yang benar
cat > order_targets.txt << EOF
POST http://localhost:8080/api/orders?user_id=2001&mode=unsafe
Content-Type: application/json

{"cart_id": "${cart_ids[0]}", "address": "Test Address", "payment_method": "cash"}
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
echo "Product 3 stock: $INITIAL_STOCK"

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
# TEST 5: MIXED WORKLOAD (SIMPLE VERSION)
# ==========================================
echo "========================================="
echo "5. SIMPLE READ WORKLOAD"
echo "========================================="

cat > simple_targets.txt << EOF
GET http://localhost:8080/api/products
GET http://localhost:8080/api/products/1
GET http://localhost:8080/api/products/search?q=product
GET http://localhost:8080/api/health
EOF

echo -e "${YELLOW}Running simple read workload (30 seconds)...${NC}"
vegeta attack -targets=simple_targets.txt \
  -rate=100 \
  -duration=30s \
  -max-workers=200 \
  -timeout=5s | vegeta report

echo ""
# ==========================================
# TEST 6: PURCHASE RACE CONDITION
# ==========================================
echo "========================================="
echo "6. PURCHASE ENDPOINT RACE CONDITION"
echo "========================================="

# Reset product 5 stock
curl -X PUT "http://localhost:8080/api/products/5/stock" \
  -H "Content-Type: application/json" \
  -d '{"stock": 50}' -s > /dev/null

cat > purchase_targets.txt << EOF
POST http://localhost:8080/api/products/5/purchase
Content-Type: application/json

{"quantity": 1}
EOF

echo -e "${YELLOW}100 users trying to purchase product 5 (stock: 50)...${NC}"
vegeta attack -targets=purchase_targets.txt \
  -rate=100 \
  -duration=1s \
  -max-workers=150 \
  -timeout=5s | vegeta report

echo ""
echo -e "${YELLOW}Checking product 5 stock after purchase race...${NC}"
FINAL_STOCK=$(curl -s "http://localhost:8080/api/products/5" | jq '.data.stock')
echo "Product 5 final stock: $FINAL_STOCK"
if [[ $FINAL_STOCK -lt 0 ]]; then
    echo -e "${RED}⚠️  RACE CONDITION DETECTED! Negative stock: $FINAL_STOCK${NC}"
elif [[ $FINAL_STOCK -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Some purchases might have failed${NC}"
else
    echo -e "${GREEN}✅ Stock correctly reduced to 0${NC}"
fi

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
echo "Check for:"
echo "1. Negative stock = Race condition"
echo "2. Success rate < 100% = Concurrency issues"
echo "3. High latency = Performance bottlenecks"

# Cleanup
rm -f cart_targets.txt cart_safe_targets.txt flash_sale_targets.txt \
      order_targets.txt simple_targets.txt purchase_targets.txt \
      flash_sale_report.txt