FROM cgr.dev/chainguard/go AS builder
COPY . /app
RUN cd /app && go build -o go-books .

FROM cgr.dev/chainguard/glibc-dynamic
COPY --from=builder /app/go-books /usr/bin/
