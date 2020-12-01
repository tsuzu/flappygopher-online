.PHONY: game server
game:
	go run *.go

server:
	go run ./server

.PHONY: wasm
wasm:
	GOOS=js GOARCH=wasm go build -o ./dist/fgo.wasm *.go
