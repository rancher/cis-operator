FROM registry.suse.com/bci/bci-busybox:15.5 as builder

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

COPY bin/cis-operator /final/usr/bin/

FROM scratch as final

COPY --from=builder /final/ /

# Aligns nobody user ID with BCI.
USER 65534:65534
ENV PATH=/usr/bin

CMD ["cis-operator"]
