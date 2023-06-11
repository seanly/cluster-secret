FROM gitpod/workspace-full
# More information: https://www.gitpod.io/docs/config-docker/

USER root

# tools
COPY --from=seanly/opsbox:toolset \
        /usr/bin/yq \
        /usr/bin/docker-compose \
        /usr/bin/helm \
        /usr/bin/kustomize \
        /usr/bin/kubectl \
        /usr/bin/fzf \
    /usr/bin/

COPY --from=seanly/toolset:kubebuilder \
        /usr/bin/kubebuilder \
    /usr/bin/

USER gitpod
