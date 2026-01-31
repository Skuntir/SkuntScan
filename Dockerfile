FROM golang:1.21-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/SkuntScan ./cmd/SkuntScan

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/SkuntScan /usr/local/bin/SkuntScan
ENTRYPOINT ["/usr/local/bin/SkuntScan"]

