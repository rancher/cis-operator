FROM alpine
COPY bin/clusterscan-operator \
     pkg/securityscan/core/templates/cisscanConfig.template \
     pkg/securityscan/core/templates/pluginConfig.template \
     pkg/securityscan/core/templates/service.template \
     /usr/bin/
CMD ["clusterscan-operator"]
