BACKEND_BIN=desktop/apps/admin-desktop/build/backend/familyvault

.PHONY: backend
backend:
	cd backend && GOOS=darwin GOARCH=arm64 go build -o ../$(BACKEND_BIN) .
	chmod +x $(BACKEND_BIN)

.PHONY: backend-dev
backend-dev:
	cd backend && go build -o ../$(BACKEND_BIN) .
	chmod +x $(BACKEND_BIN)

.PHONY: app-dev
app-dev: backend-dev
	cd desktop/apps/admin-desktop && pnpm i && pnpm dev

.PHONY: app-build
app-build: backend
	cd desktop/apps/admin-desktop && pnpm build && pnpm electron:build

.PHONY: test
test:
	cd backend && go test ./...
	cd desktop/apps/admin-desktop && pnpm test

.PHONY: clean
clean:
	rm -rf desktop/apps/admin-desktop/build/
	rm -rf desktop/apps/admin-desktop/dist/
	rm -rf desktop/apps/admin-desktop/node_modules/

.PHONY: install
install:
	cd desktop/apps/admin-desktop && pnpm i

.PHONY: dev
dev: app-dev

.PHONY: build
build: app-build

.PHONY: all
all: test build