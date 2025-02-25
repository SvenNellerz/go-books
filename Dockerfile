# Build stage
FROM golang:1.24.0-alpine3.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o books-app .

# Run stage
FROM scratch
COPY --from=builder /app/books-app /books-app
EXPOSE 8080
ENTRYPOINT ["/books-app"]