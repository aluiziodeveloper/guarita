# As imagens de base são fixadas pelo digest, como os servidores no
# Capítulo 20: uma etiqueta pode passar a apontar para outro conteúdo.
ARG GO_IMAGE=golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8
ARG RUN_IMAGE=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

# #region deploy-lab-image
FROM ${GO_IMAGE} AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY examples/ ./examples/
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" \
      -o /out/lab-agent ./cmd/lab-agent \
    && go build -trimpath -ldflags="-s -w" \
      -o /out/lab-mcp ./cmd/lab-poisoned-mcp \
    && go build -trimpath -ldflags="-s -w" \
      -o /out/lab-files ./cmd/lab-files-mcp \
    && go build -trimpath -ldflags="-s -w" \
      -o /out/lab-load ./cmd/lab-load

FROM ${RUN_IMAGE}

COPY --from=build /out/lab-agent /lab-agent
COPY --from=build /out/lab-mcp /lab-mcp
COPY --from=build /out/lab-files /lab-files
COPY --from=build /out/lab-load /lab-load
# #endregion
