FROM registry.suse.com/bci/bci-busybox:15.5

COPY pkg/ pkg/

# Ensure 65535 can access the templates in
# pkg/securityscan/core/templates
RUN chmod -R +xr pkg/

COPY bin/cis-operator /usr/bin/

USER 65535:65535

CMD ["cis-operator"]
