FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS=linux TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/server /server
USER nonroot:nonroot
EXPOSE {{ cookiecutter.port }}
ENTRYPOINT ["/server"]
