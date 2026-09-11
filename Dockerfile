FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/k8sgames-server ./cmd/k8sgames-server

FROM alpine:3.22
ARG KUBECTL_VERSION=v1.35.0
ARG TARGETARCH
RUN apk add --no-cache ca-certificates curl \
    && curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x /usr/local/bin/kubectl
COPY --from=build /out/k8sgames-server /usr/local/bin/k8sgames-server
ENV HOME=/tmp
USER 65532:65532
ENTRYPOINT ["k8sgames-server"]
