# build stage
FROM golang:1.25.7-alpine3.23 AS builder

ARG VERSION=dev
ARG BRANCH=unknown
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-w -s \
      -X github.com/prometheus/common/version.Version=${VERSION} \
      -X github.com/prometheus/common/version.Revision=${COMMIT} \
      -X github.com/prometheus/common/version.BuildDate=${BUILD_DATE} \
      -X github.com/prometheus/common/version.Branch=${BRANCH}" \
    -o /jobstats_cgroup_exporter \
    ./cmd/jobstats_cgroup_exporter

# final stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates

COPY --from=builder /jobstats_cgroup_exporter /jobstats_cgroup_exporter

# NOTE: This exporter reads the host's cgroup v2 filesystem and /proc. When run
# in a container it must be given access to the host, e.g.:
#   docker run --pid=host \
#     -v /sys/fs/cgroup:/sys/fs/cgroup:ro \
#     jobstats_cgroup_exporter
# Reading other users' cgroups and process info requires appropriate privileges
# (typically running as root on the host), so no unprivileged USER is set here.

EXPOSE 9306

ENTRYPOINT ["/jobstats_cgroup_exporter"]
