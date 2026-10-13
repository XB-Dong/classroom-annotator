# Use verified Go artifacts for the requested Linux architecture.
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:a4672c0cb26fbdde88e38fa2dfb6c681942306680e41e4378b28770b6e79ee91
FROM ${RUNTIME_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 app && useradd --uid 10001 --gid 10001 --no-create-home app \
    && mkdir /data && chown app:app /data
COPY deploy/artifacts/backend-server /usr/local/bin/backend-server
COPY deploy/artifacts/migrate /usr/local/bin/migrate
COPY deploy/artifacts/import-sqlite /usr/local/bin/import-sqlite
COPY deploy/artifacts/seed-samples /usr/local/bin/seed-samples
COPY deploy/artifacts/bootstrap-user /usr/local/bin/bootstrap-user
RUN chmod 0555 /usr/local/bin/backend-server /usr/local/bin/migrate /usr/local/bin/import-sqlite /usr/local/bin/seed-samples /usr/local/bin/bootstrap-user
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/backend-server"]
