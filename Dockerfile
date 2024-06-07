FROM golang:1.22.3 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN  go build -o judger .
    
FROM node:20.14.0
WORKDIR /app

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y

COPY --from=builder /app/judger /app/
COPY --from=builder /app/languages/ /app/languages/

CMD ["/app/judger"]
ENTRYPOINT ["/app/judger"]
