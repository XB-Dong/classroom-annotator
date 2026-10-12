# Build the Linux amd64 Go artifact before invoking Docker; no cloud Go pull.
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:a4672c0cb26fbdde88e38fa2dfb6c681942306680e41e4378b28770b6e79ee91
FROM ${RUNTIME_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 app && useradd --uid 10001 --gid 10001 --no-create-home app \
    && mkdir /data && chown app:app /data
COPY --chmod=0555 deploy/artifacts/backend-server /usr/local/bin/backend-server
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/backend-server"]
