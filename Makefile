APP_NAME=sentrygo
MAIN_PATH=cmd/api/main.go
BIN_DIR=bin

run:
	go run $(MAIN_PATH)

build:
	go build -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PATH)

build-linux:
	set GOOS=linux&& set GOARCH=amd64&& go build -o $(BIN_DIR)/$(APP_NAME)_linux $(MAIN_PATH)

tidy:
	go mod tidy

clean:
	if exist $(BIN_DIR) rmdir /s /q $(BIN_DIR)