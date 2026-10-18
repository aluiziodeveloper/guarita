# Imagem de produção da Guarita (Capítulo 21). As imagens de base vêm
# pelo digest, como no Dockerfile do laboratório.
ARG GO_IMAGE=golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8
ARG RUN_IMAGE=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

# #region deploy-guarita-image
FROM ${GO_IMAGE} AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY policies/ ./policies/
ENV CGO_ENABLED=0
RUN go build -trimpath -buildvcs=false \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/guarita ./cmd/guarita

# Sem shell, sem gerenciador de pacotes, usuário 65532: só o binário
# estático, os certificados raiz e as políticas.
FROM ${RUN_IMAGE}
COPY --from=build /out/guarita /guarita
COPY --from=build /src/policies /policies
USER 65532:65532
EXPOSE 8443 9090
ENTRYPOINT ["/guarita"]
# #endregion
