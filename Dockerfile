FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ibp-collator ./src/IBPCollator.go

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/ibp-collator /app/ibp-collator

ENTRYPOINT ["/app/ibp-collator", "-config", "/app/config/ibpcollator.json"]
