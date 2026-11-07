FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/app ./cmd/trip-service


FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=build /bin/app /app
USER nonroot
ENTRYPOINT ["/app"]