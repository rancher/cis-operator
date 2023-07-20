FROM registry.suse.com/bci/bci-minimal:15.5

COPY pkg/ pkg/
COPY bin/cis-operator /usr/bin/

USER 65535:65535

CMD ["cis-operator"]
