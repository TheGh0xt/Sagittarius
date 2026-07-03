instructions:
    echo "To build, run 'just build'."
    echo "To run, run 'just run'."

build:
    go build -o bin/sagittarius-mcp cmd/sagittarius-mcp/main.go
    echo "Built successfully!"

run-http:
    ./bin/sagittarius-mcp --transport=http --port=8080
    echo "Running successfully!"

run-sse:
    ./bin/sagittarius-mcp --transport=sse --port=8080
    echo "Running SSE successfully!"
