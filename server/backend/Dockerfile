FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /zcopy-server .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /zcopy-server .
COPY config/config.yaml /app/config/config.yaml

EXPOSE 8890

CMD ["./zcopy-server"]
