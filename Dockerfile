# This is a multi-stage Dockerfile and requires >= Docker 17.05
# https://docs.docker.com/engine/userguide/eng-image/multistage-build/
FROM swift:6.3-jammy AS builder

WORKDIR /build

# Resolve dependencies before copying source, so source-only changes don't invalidate the
# downloaded-dependencies layer.
COPY Package.swift Package.resolved* ./
RUN swift package resolve

COPY . .
# --jobs 2: this app's release build (Fluent + FluentMongoDriver + Redis + the platform's
# largest controller/migration set) was silently failing on every CI run - swift build
# stalls mid-compile and the container gets killed without an explicit error, then the
# runtime stage's COPY fails with "not found" since the binary was never produced. Reproduced
# locally with plenty of headroom (32GB, succeeds in ~6 min) but never once succeeded on
# GitHub's ~7GB standard runners - default (unbounded) compiler parallelism spikes peak
# memory past that ceiling for this app's dependency graph specifically; the platform's
# smaller Vapor apps (auth-web, admin-web) build fine unbounded on the same runner class.
RUN swift build -c release --static-swift-stdlib --jobs 2

FROM swift:6.3-jammy-slim

ARG USERNAME=sweetrpg
ARG BUILD_NUMBER=unset
ARG BUILD_JOB=unset
ARG BUILD_SHA=unset
ARG BUILD_DATE=unset
ARG BUILD_VERSION=unset

RUN useradd --user-group --create-home --system --skel /dev/null $USERNAME

WORKDIR /app

RUN mkdir -p /app/bin /app/config
COPY --from=builder /build/.build/release/App /app/bin/

RUN echo "{\"number\":\"${BUILD_NUMBER}\",\"job\":\"${BUILD_JOB}\",\"sha\":\"${BUILD_SHA}\",\"date\":\"${BUILD_DATE}\",\"version\":\"${BUILD_VERSION}\"}" > /app/config/build-info.json
RUN chown -R ${USERNAME}:${USERNAME} /app

ENV PORT="8080"
ENV REDIS_HOST=""
ENV REDIS_PORT="6379"
ENV VERSION=${BUILD_VERSION}

EXPOSE 8080

USER ${USERNAME}

ENTRYPOINT ["/app/bin/App"]
CMD ["serve"]
