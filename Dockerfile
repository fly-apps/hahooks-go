FROM golang:1.21 as builder

WORKDIR /app
COPY . /app
RUN GOOS=linux GOARCH=amd64 go build -o /app/hahooks

FROM debian:bookworm-slim

COPY --from=builder /app/hahooks /usr/local/bin/hahooks

EXPOSE 8080

CMD ["hahooks"]
