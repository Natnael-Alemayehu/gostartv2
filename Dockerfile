FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags "-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o main cmd/api/main.go

FROM alpine:3.22 AS prod

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=build /app/main /app/main

USER app

EXPOSE 8080

CMD ["./main"]
