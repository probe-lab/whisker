build-all:
    mkdir -p dist
    for dir in cmd/*/; do \
        go build -o dist/$(basename "$dir") ./"$dir"; \
    done
