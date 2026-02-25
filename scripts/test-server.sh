#!/bin/bash
# test-server.sh - Run the crafting server test suite
# This script builds and runs the test tools for the crafting server.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo -e "${GREEN}Building test-tools...${NC}"
go build -o bin/test-tools ./cmd/test-tools

if [ ! -f "bin/test-tools" ]; then
    echo -e "${RED}Failed to build test-tools${NC}"
    exit 1
fi

echo -e "${GREEN}Running tests...${NC}\n"

# Set database path if not already set
if [ -z "$CRAFTING_DB" ]; then
    if [ -f "database/crafting.db" ]; then
        export CRAFTING_DB="database/crafting.db"
    elif [ -f "data/crafting/crafting.db" ]; then
        export CRAFTING_DB="data/crafting/crafting.db"
    else
        echo -e "${YELLOW}Warning: No database found. Set CRAFTING_DB environment variable.${NC}"
        echo -e "${YELLOW}Attempting to run tests anyway...${NC}\n"
    fi
fi

./bin/test-tools "$@"
EXIT_CODE=$?

echo ""

if [ $EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Some tests failed.${NC}"
fi

exit $EXIT_CODE
