#!/bin/bash
# Manual test script for TUI fixes
# Run this in an interactive terminal to verify the attach and switch commands work

set -e

# Create test directory
TEST_DIR="/tmp/st-tui-test-$$"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

echo "=== Creating test repo in $TEST_DIR ==="
git init
git config user.email "test@test.com"
git config user.name "Test User"
echo "initial" > file.txt
git add .
git commit -m "initial commit"

echo ""
echo "=== Step 1: Create m1 with st ==="
st new m1
echo "Current branch: $(git branch --show-current)"
echo "Graph contents:"
cat .git/stack/graph.json | head -20

echo ""
echo "=== Step 2: Create m2 manually ==="
git checkout main
git checkout -b m2
echo "m2" >> file.txt
git add .
git commit -m "m2 commit"

echo ""
echo "=== Step 3: Create m3 manually ==="
git checkout -b m3
echo "m3" >> file.txt
git add .
git commit -m "m3 commit"

echo ""
echo "=== Step 4: Check graph before attach ==="
echo "Branches:"
git branch -a
echo ""
echo "Graph (should only have m1):"
cat .git/stack/graph.json | head -20

echo ""
echo "=== Step 5: Test st attach (interactive) ==="
echo "Instructions:"
echo "1. Use arrow keys to select 'm2'"
echo "2. Press Enter to select it as parent of m3"
echo "3. You should then be prompted to attach m2"
echo "4. Select 'main' or 'm1' as parent of m2"
echo ""
read -p "Press Enter to start attach..."
st attach

echo ""
echo "=== Step 6: Verify graph after attach ==="
echo "Graph contents:"
cat .git/stack/graph.json

echo ""
echo "=== Step 7: Test st switch (interactive) ==="
echo "Instructions:"
echo "1. Use arrow keys to navigate"
echo "2. Press Enter to select a branch"
echo "3. You should switch to that branch"
echo ""
read -p "Press Enter to start switch..."
st switch

echo ""
echo "=== Test Complete ==="
echo "Current branch: $(git branch --show-current)"
echo ""
echo "Cleanup: rm -rf $TEST_DIR"
