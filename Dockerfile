# One image per Go service: docker build --build-arg SERVICE=ledger .
FROM golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY gen ./gen
COPY internal ./internal
COPY services ./services
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./services/${SERVICE}

FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
 && wget -qO /etc/ssl/sf-class2-root.crt https://certs.secureserver.net/repository/sf-class2-root.crt
COPY --from=build /out/app /app
USER 65532
ENTRYPOINT ["/app"]
