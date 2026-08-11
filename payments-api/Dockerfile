# Build a small, reproducible Go binary.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/crnakobra-payments-api .

# Nginx serves the HTML; this container only exposes the Go API.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/crnakobra-payments-api /crnakobra-payments-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/crnakobra-payments-api"]
