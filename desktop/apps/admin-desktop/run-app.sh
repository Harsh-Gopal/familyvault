#!/bin/bash

# FamilyVault Desktop App Runner
# This script helps you run the app in different modes

set -e

echo "🏠 FamilyVault Desktop App Runner"
echo "================================="

# Check if we're in the right directory
if [ ! -f "package.json" ]; then
    echo "❌ Error: Please run this script from the desktop/apps/admin-desktop directory"
    exit 1
fi

# Function to check if backend is running
check_backend() {
    if curl -s http://localhost:8000/health > /dev/null 2>&1; then
        echo "✅ Backend is running"
        return 0
    else
        echo "⚠️  Backend is not running"
        return 1
    fi
}

# Function to start backend
start_backend() {
    echo "🚀 Starting backend..."
    if [ -f "build/backend/familyvault" ]; then
        ./build/backend/familyvault &
        BACKEND_PID=$!
        echo "Backend started with PID: $BACKEND_PID"
        sleep 3
    elif [ -f "../../../backend/familyvault" ]; then
        cd ../../../backend
        ./familyvault &
        BACKEND_PID=$!
        echo "Backend started with PID: $BACKEND_PID"
        cd - > /dev/null
        sleep 3
    else
        echo "❌ Backend binary not found. Please run 'make build' from the root directory first."
        exit 1
    fi
}

# Function to run in development mode
run_dev() {
    echo "🔧 Running in development mode..."
    if ! check_backend; then
        start_backend
    fi
    pnpm dev
}

# Function to run built app
run_built() {
    echo "📦 Running built application..."
    if [ -d "release/mac-arm64/FamilyVault.app" ]; then
        if ! check_backend; then
            start_backend
        fi
        open "release/mac-arm64/FamilyVault.app"
    else
        echo "❌ Built app not found. Please run 'pnpm electron:build' first."
        exit 1
    fi
}

# Function to open DMG
open_dmg() {
    echo "💿 Opening DMG file..."
    if [ -f "release/FamilyVault-1.0.0-arm64.dmg" ]; then
        open "release/FamilyVault-1.0.0-arm64.dmg"
    else
        echo "❌ DMG file not found. Please run 'pnpm electron:build' first."
        exit 1
    fi
}

# Function to build everything
build_all() {
    echo "🔨 Building application..."
    echo "1. Building backend..."
    cd ../../..
    make build
    cd desktop/apps/admin-desktop
    echo "2. Building frontend..."
    pnpm build
    echo "3. Building Electron app..."
    pnpm electron:build
    echo "✅ Build complete!"
}

# Function to run tests
run_tests() {
    echo "🧪 Running tests..."
    echo "1. Running unit tests..."
    pnpm test
    echo "2. Running E2E tests..."
    pnpm e2e
}

# Function to show help
show_help() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  dev      - Run in development mode (default)"
    echo "  built    - Run the built application"
    echo "  dmg      - Open the DMG file"
    echo "  build    - Build everything (backend + frontend + electron)"
    echo "  test     - Run all tests"
    echo "  help     - Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0           # Run in development mode"
    echo "  $0 dev       # Run in development mode"
    echo "  $0 built     # Run built app"
    echo "  $0 dmg       # Open DMG"
    echo "  $0 build     # Build everything"
}

# Parse command line arguments
case "${1:-dev}" in
    "dev")
        run_dev
        ;;
    "built")
        run_built
        ;;
    "dmg")
        open_dmg
        ;;
    "build")
        build_all
        ;;
    "test")
        run_tests
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        echo "❌ Unknown command: $1"
        show_help
        exit 1
        ;;
esac