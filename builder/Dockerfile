FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/builder .

FROM debian:trixie-slim

# curl/bash: railpack インストーラ用 (インストーラは bash スクリプトのため sh では動かない)
# ca-certificates: git clone (https) / railpack ダウンロード用
RUN apt-get update && apt-get install -y --no-install-recommends \
      curl \
      bash \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# buildctl (BuildKit のクライアント CLI)
# Dockerfile を使ったビルド (dockerfile.v0 フロントエンド) 用。
# buildkitd 本体は別コンテナ (docker-compose の buildkit サービス) で動かすため、
# ここでは buildctl バイナリのみを取得する。
COPY --from=moby/buildkit:v0.27.0 /usr/bin/buildctl /usr/local/bin/buildctl

# Railpack のインストール
# railpack は内部で mise (glibcバイナリ) を利用するため、glibcベースの Debian を使用する
# (musl の Alpine では glibc 互換レイヤーを入れても動かないシンボルがあるため不採用)
ARG RAILPACK_VERSION=0.29.0
RUN mkdir -p /usr/local/railpack/bin && \
    curl -sSL https://railpack.com/install.sh -o /tmp/install.sh && \
    RAILPACK_VERSION=${RAILPACK_VERSION} bash /tmp/install.sh --bin-dir /usr/local/railpack/bin --force && \
    rm /tmp/install.sh && \
    /usr/local/railpack/bin/railpack --version

ENV PATH="/usr/local/railpack/bin:${PATH}"

WORKDIR /app

COPY --from=build /out/builder /usr/local/bin/builder

ENTRYPOINT ["/usr/local/bin/builder"]
