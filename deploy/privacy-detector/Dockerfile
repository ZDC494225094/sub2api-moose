# syntax=docker/dockerfile:1.7
# sub2api privacy detector: AstrLink's privacy-worker and the AstrLink Guard
# model (CPU INT8) behind a small authenticated HTTP API.
#
#   docker build -t sub2api-privacy-detector deploy/privacy-detector
#
# Every external input is pinned: the AstrLink source commit, ONNX Runtime by
# checksum and each model file by revision and SHA-256.

ARG ASTRLINK_COMMIT=8e8cf338c51c1132d8aae22e0d424afeecadc7c3
ARG GUARD_REVISION=7216b6a3b897ef903d4b1ba367ae6fb95b0fa5e4
ARG HF_ENDPOINT=https://huggingface.co

# AstrLink privacy worker (Apache-2.0), built natively for the target platform.
FROM rust:1.97.1-bookworm AS worker
ARG ASTRLINK_COMMIT
WORKDIR /src
RUN git init -q astrlink \
    && cd astrlink \
    && git remote add origin https://github.com/Calcium-Ion/AstrLink.git \
    && git fetch -q --depth 1 origin "$ASTRLINK_COMMIT" \
    && git checkout -q FETCH_HEAD
WORKDIR /src/astrlink/apps/privacy-worker
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    cargo build --locked --release \
    && install -D target/release/astrlink-privacy-worker /out/astrlink-privacy-worker \
    && install -D /src/astrlink/LICENSE /out/LICENSE.astrlink

# ONNX Runtime 1.23.2, the version the worker loads, from Microsoft's NuGet package.
FROM debian:12-slim AS onnxruntime
ARG TARGETARCH
ADD --checksum=sha256:25fe172f7fcdf34f2b5b02c8b997f6b88abc282956849d4af4335af23d0c4e4a \
    https://api.nuget.org/v3-flatcontainer/microsoft.ml.onnxruntime/1.23.2/microsoft.ml.onnxruntime.1.23.2.nupkg /tmp/onnxruntime.nupkg
RUN apt-get update \
    && apt-get install -y --no-install-recommends unzip \
    && rm -rf /var/lib/apt/lists/* \
    && case "${TARGETARCH:-amd64}" in \
        amd64) rid=linux-x64 ;; \
        arm64) rid=linux-arm64 ;; \
        *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && mkdir -p /tmp/ort /out \
    && unzip -q /tmp/onnxruntime.nupkg "runtimes/$rid/native/libonnxruntime.so" LICENSE -d /tmp/ort \
    && cp "/tmp/ort/runtimes/$rid/native/libonnxruntime.so" /out/libonnxruntime.so.1.23.2 \
    && tr -d '\r' < /tmp/ort/LICENSE > /out/LICENSE.onnxruntime

# AstrLink Guard CPU INT8 (Apache-2.0), the model AstrLink recommends.
FROM debian:12-slim AS model
ARG GUARD_REVISION
ARG HF_ENDPOINT
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /out/model
RUN set -eu; \
    base="$HF_ENDPOINT/QuantumNous/astrlink-guard/resolve/$GUARD_REVISION"; \
    for f in int8/model_int8.onnx int8/model_int8.onnx_data int8/config.json int8/tokenizer.json int8/tokenizer_config.json LICENSE; do \
        mkdir -p "$(dirname "$f")"; \
        curl -fsSL --retry 5 --retry-delay 3 -o "$f" "$base/$f"; \
    done; \
    printf '%s  %s\n' \
        0a070d3113615e33ba3b84d6bd486f663a377c04c4b056e7381732179f51832a int8/model_int8.onnx \
        0ba34216ceead01e0ae2dfd8e46d52b67e7aee80b04f722d9b758613a91c0067 int8/model_int8.onnx_data \
        2c20003590d642e273c03c4c7c82d9849fa75f64bf29a71ec0a0c9a20ba4a150 int8/config.json \
        0cf3b235ef125658015a6c8026a5fe6f28088bf825101a990ec18a10200feef4 int8/tokenizer.json \
        3a7cf4d70bb7c0376f18cc6ad23c143853de5b4478084c414d5dbbd35ea5c589 int8/tokenizer_config.json \
        c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4 LICENSE \
        | sha256sum -c -
COPY astrlink-model.json /out/model/astrlink-model.json

# The HTTP front end (stdlib only).
FROM golang:1.26-bookworm AS server
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/privacy-detector .

FROM debian:12-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends tini \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 detector \
    && useradd --system --uid 10001 --gid detector --no-create-home --shell /usr/sbin/nologin detector
# The worker finds ONNX Runtime beside its own executable.
COPY --from=worker /out/astrlink-privacy-worker /opt/astrlink/
COPY --from=onnxruntime /out/libonnxruntime.so.1.23.2 /opt/astrlink/
COPY --from=model /out/model/ /opt/astrlink/model/
COPY --from=server /out/privacy-detector /usr/local/bin/privacy-detector
COPY --from=worker /out/LICENSE.astrlink /usr/share/doc/privacy-detector/LICENSE.astrlink
COPY --from=onnxruntime /out/LICENSE.onnxruntime /usr/share/doc/privacy-detector/LICENSE.onnxruntime
USER detector
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 CMD ["privacy-detector", "healthcheck"]
ENTRYPOINT ["tini", "--", "privacy-detector"]
