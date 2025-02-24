# Build stage
FROM golang:1.20-alpine AS builder
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
