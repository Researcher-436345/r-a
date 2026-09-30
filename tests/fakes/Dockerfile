# Build context: ./tests (go.mod of the tests module).
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/fake-llm ./fakes/cmd/fake-llm && \
    CGO_ENABLED=0 go build -trimpath -o /out/fake-scholar ./fakes/cmd/fake-scholar

FROM alpine:3.20
RUN apk add --no-cache wget ca-certificates
COPY --from=build /out/fake-llm /usr/local/bin/fake-llm
COPY --from=build /out/fake-scholar /usr/local/bin/fake-scholar
EXPOSE 80 443 8093 8094
CMD ["fake-llm"]
