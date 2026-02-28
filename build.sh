export CGO_ENABLED=1
export GOOS=linux
export GOARCH=arm64
export CC=aarch64-linux-gnu-gcc
go build -buildmode=c-shared -o libembedhttplua.so main.go