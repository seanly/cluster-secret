FROM gitpod/workspace-full
# More information: https://www.gitpod.io/docs/config-docker/

RUN brew install kustomize kubebuilder kubectl
