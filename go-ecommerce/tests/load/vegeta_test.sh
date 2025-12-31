#!/bin/bash

echo "========================================="
echo "🚀 GO PRODUCT API LOAD TEST"
echo "========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if server is running
echo -e "${YELLOW}Checking if server is running...${NC}"
if ! curl -s http://localhost:8080/api/health > /dev/null; then
    echo -e "${RED}❌ Server not running on port 8080${NC}"
    echo "Start the server first: go run cmd/web/main.go"
    exit 1
fi

echo -e "${GREEN}✅ Server is running${NC}"
echo ""

# Create targets file for Vegeta
echo "Creating test targets..."
cat > targets.txt << EOF
GET http://localhost:8080/api/products
GET http://localhost:8080/api/products/1
GET http://localhost:8080/api/products/100
GET http://localhost:8080/api/products/500
GET http://localhost:8080/api/products/search?q=product
GET http://localhost:8080/api/products/search?category=Electronics&min_price=100&max_price=500
EOF

echo "========================================="
echo "1. BASELINE TEST (100 requests)"
echo "========================================="
echo -e "${YELLOW}Testing with 10 concurrent users, 100 total requests...${NC}"
echo "GET http://localhost:8080/api/products" | vegeta attack \
  -rate=10 \
  -duration=10s \
  -timeout=5s | vegeta report

echo ""
echo "========================================="
echo "2. MEDIUM LOAD TEST (1000 requests)"
echo "========================================="
echo -e "${YELLOW}Testing with 50 concurrent users, 1000 total requests...${NC}"
echo "GET http://localhost:8080/api/products" | vegeta attack \
  -rate=50 \
  -duration=20s \
  -timeout=5s | vegeta report

echo ""
echo "========================================="
echo "3. HIGH LOAD TEST (Product List)"
echo "========================================="
echo -e "${YELLOW}Testing product listing with 200 RPS for 30 seconds...${NC}"
vegeta attack \
  -targets=targets.txt \
  -rate=200 \
  -duration=30s \
  -max-workers=500 \
  -timeout=10s | vegeta report > report.txt

echo -e "${GREEN}✅ Detailed report saved to report.txt${NC}"

echo ""
echo "========================================="
echo "4. RACE CONDITION TEST (Purchase)"
echo "========================================="
echo -e "${YELLOW}Testing 100 concurrent purchases of same product...${NC}"
echo -e "${RED}⚠️  This will test race conditions on stock update${NC}"

# Create purchase targets
cat > purchase_targets.txt << EOF
POST http://localhost:8080/api/products/1/purchase
Content-Type: application/json
{"quantity": 1}

POST http://localhost:8080/api/products/1/purchase
Content-Type: application/json
{"quantity": 1}

POST http://localhost:8080/api/products/1/purchase
Content-Type: application/json
{"quantity": 1}
EOF

vegeta attack \
  -targets=purchase_targets.txt \
  -rate=100 \
  -duration=2s \
  -max-workers=200 \
  -timeout=5s | vegeta report

echo ""
echo "========================================="
echo "5. CHECKING RESULTS"
echo "========================================="

# Check final stock
echo -e "${YELLOW}Checking product stock after purchases...${NC}"
curl -s http://localhost:8080/api/products/1 | jq '.data.stock'

echo ""
echo -e "${GREEN}✅ Load test completed!${NC}"

# Cleanup
rm -f targets.txt purchase_targets.txt results.bin