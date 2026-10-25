FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/challenger ./cmd/challenger
RUN mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/challenger /challenger
COPY --from=build --chown=65532:65532 /out/data /data
ENV DB_PATH=/data/challenger.db
VOLUME /data
ENTRYPOINT ["/challenger"]
