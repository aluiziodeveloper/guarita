# #region deploy-lab-image
FROM golang:1.27.2 AS build

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
      -o /out/lab-files ./cmd/lab-files-mcp

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/lab-agent /lab-agent
COPY --from=build /out/lab-mcp /lab-mcp
COPY --from=build /out/lab-files /lab-files
# #endregion
