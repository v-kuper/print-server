FROM --platform=linux/amd64 golang:1.25-bookworm AS build

WORKDIR /src/server

COPY go.mod ./
COPY internal/fptr10 ./internal/fptr10
RUN go mod download

COPY . ./

ENV CGO_ENABLED=1
RUN go build -trimpath -o /out/atol-server ./cmd/atol-server

FROM --platform=linux/amd64 debian:bookworm-slim AS runtime-base

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libstdc++6 \
    && ln -sf /usr/lib/x86_64-linux-gnu/libcrypto.so.3 /usr/lib/x86_64-linux-gnu/libcrypto.so \
    && ln -sf /usr/lib/x86_64-linux-gnu/libssl.so.3 /usr/lib/x86_64-linux-gnu/libssl.so \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /data

COPY --from=build /out/atol-server /usr/local/bin/atol-server
COPY assets/ /opt/atol-server/assets/

ENV HTTP_ADDR=:8080
ENV SETTINGS_PATH=/data/settings.json
ENV ASSETS_PATH=/opt/atol-server/assets
ENV GODEBUG=asyncpreemptoff=1

EXPOSE 8080
VOLUME ["/data"]

FROM runtime-base AS runtime-plain

RUN mkdir -p /opt/atol/lib
COPY driver/linux-amd64/ /opt/atol/lib/
RUN test -f /opt/atol/lib/libfptr10.so \
    || (echo "Put ATOL Driver 10.10.7.0 Linux x64 files into driver/linux-amd64 before building runtime-plain" >&2; exit 1)

ENV ATOL_LIBRARY_PATH=/opt/atol/lib
ENV LD_LIBRARY_PATH=/opt/atol/lib

ENTRYPOINT ["atol-server"]

FROM runtime-base AS runtime-uem

COPY installer/deb/libfptr10_10.10.8.0_amd64_uem.deb /tmp/libfptr10_uem.deb
RUN dpkg-deb -x /tmp/libfptr10_uem.deb / \
    && dpkg-deb -x /usr/share/uem/uema.deb / \
    && dpkg-deb -x /usr/share/uem/uemu.deb / \
    && rm -f /tmp/libfptr10_uem.deb \
    && rm -rf /usr/share/uem

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENV ATOL_LIBRARY_PATH=/usr/lib
ENV LD_LIBRARY_PATH=/usr/lib:/usr/lib/fptr10

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["atol-server"]
