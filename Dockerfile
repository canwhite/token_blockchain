# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata && \
    go env -w GOPROXY=https://goproxy.cn,direct && \
    go env -w GOSUMDB=off

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./
COPY api/ api/
COPY service/ service/
COPY middleware/ middleware/
COPY database/ database/
COPY eventstore/ eventstore/
COPY utils/ utils/
COPY security/ security/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o token_blockchain .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

ENV TZ=Asia/Shanghai

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/token_blockchain .

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

CMD ["./token_blockchain"]