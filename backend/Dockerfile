# File: backend/Dockerfile
FROM golang:1.26-alpine AS builder

WORKDIR /app

# The context is now the 'backend' directory, so we just copy directly
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_NAME 
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/${APP_NAME}/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /bin/app .

# Using an env variable for port is a 12-factor best practice
ENV PORT=8080
EXPOSE 8080

CMD ["./app"]