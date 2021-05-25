FROM golang AS builder

RUN mkdir /app
ADD . /app
WORKDIR /app

RUN CGO_ENABLED=0 GOOS=linux go build -o a10-http-error-monitor ./...

FROM alpine:latest as production

COPY --from=builder /app .
CMD ["./a10-http-error-monitor"]
