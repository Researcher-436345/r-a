FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/identity ./cmd/identity && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/catalog ./cmd/catalog && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/library ./cmd/library && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/annotations ./cmd/annotations && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/assistant ./cmd/assistant && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/feed ./cmd/feed && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/searchapi ./cmd/searchapi && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/translator ./cmd/translator

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=build /out/gateway /usr/local/bin/gateway
COPY --from=build /out/identity /usr/local/bin/identity
COPY --from=build /out/catalog /usr/local/bin/catalog
COPY --from=build /out/library /usr/local/bin/library
COPY --from=build /out/annotations /usr/local/bin/annotations
COPY --from=build /out/assistant /usr/local/bin/assistant
COPY --from=build /out/feed /usr/local/bin/feed
COPY --from=build /out/searchapi /usr/local/bin/searchapi
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/translator /usr/local/bin/translator
EXPOSE 8000 8090 8101 8102 8103 8104 8105 8106 8107
CMD ["gateway"]
