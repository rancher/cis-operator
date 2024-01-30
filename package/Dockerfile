# Image that provides cross compilation tooling.
FROM --platform=$BUILDPLATFORM rancher/mirrored-tonistiigi-xx:1.3.0 as xx

FROM registry.suse.com/bci/golang:1.21 AS builder

# There is no real need for containers to fully comply with the
# Filesystem Hierarchy Standard (FHS). However, some applications
# could malfunction if some specific basic dirs are not available.
# Therefore, create top level structure.
#
# https://refspecs.linuxfoundation.org/FHS_3.0/fhs/index.html
RUN mkdir -p /final/boot && \
    mkdir -p /final/etc && \
    mkdir -p /final/home && \
    mkdir -p /final/lib && \
    mkdir -p /final/lib64 && \
    mkdir -p /final/media && \
    mkdir -p /final/mnt && \
    mkdir -p /final/opt && \
    mkdir -p /final/run && \
    mkdir -p /final/usr/sbin && \
    mkdir -p /final/var/lib/nobody

# Some dirs require very specific permissions.
RUN install -dv -m 0750 /final/root && \
    install -dv -m 1777 /final/tmp /final/var/tmp

# Keep name search configured in line with BCI.
RUN cp /etc/nsswitch.conf /final/etc

# Differs from BCI, by removing /bin/sh from root:
RUN echo "root:x:0:0:root:/root:/usr/bin/false\nnobody:x:65534:65534:nobody:/var/lib/nobody:/usr/bin/false" > /final/etc/passwd

RUN cp /etc/shadow /final/etc
RUN cp /etc/group /final/etc

WORKDIR /src
COPY go.sum \
     go.mod \
     Makefile \
     /src
RUN go mod download

COPY pkg /src/pkg
COPY hack /src/hack
COPY main.go /src

# By setting the version as an argument, we can avoid running the version logic 
# a second time (inside the Docker build process). Therefore, removing the need
# to access the .git dir.
ARG VERSION
RUN VERSION=${VERSION} TARGET_BIN=/final/usr/bin/cis-operator make build

# Ensures that the binary that was built was cross-compiled correctly
# and is valid on the target platform.
COPY --from=xx \
    /usr/bin/xx-verify \
    /usr/bin/xx-info \
    /usr/bin
RUN mkdir -p /run/lock
RUN xx-verify --static /final/usr/bin/cis-operator

FROM scratch as final

COPY --from=builder /final/ /

# Aligns nobody user ID with BCI.
USER 65534:65534
ENV PATH=/usr/bin

CMD ["cis-operator"]
