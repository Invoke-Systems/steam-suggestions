# syntax=docker/dockerfile:1
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# Static Go binaries need only CA roots — avoid a full Debian userspace (CVE surface).
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/server /out/worker /app/
VOLUME /app/data
EXPOSE 3847
ENV PORT=3847
CMD ["/app/server"]
