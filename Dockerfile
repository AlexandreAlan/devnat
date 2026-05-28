# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/alexandrealan/devnat/internal/buildinfo.Version=${VERSION}" \
    -o /out/devnat ./cmd/devnat

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/devnat /usr/local/bin/devnat
ENV DEVNAT_CERT_DIR=/data/certmagic
EXPOSE 443
ENTRYPOINT ["devnat"]
CMD ["relay"]
