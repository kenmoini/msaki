FROM registry.access.redhat.com/ubi9/nodejs-22:9.7
WORKDIR /opt/app-root/src
USER 0

# Labels for OpenShift/Kubernetes
LABEL name="msaki-devcontainer" \
      summary="Model Swiss Army Knife Interface DevContainer" \
      description="Devcontainer for building MSAKI, a unified interface for managing and proxying AI model endpoints" \
      maintainer="Ken Moini" \
      version="1.0" \
      io.k8s.display-name="MSAKI-DevContainer" \
      io.k8s.description="Devcontainer for Model Swiss Army Knife Interface - Proxy and manage AI model endpoints" \
      io.openshift.tags="ai,llm,proxy,openai,ollama,devcontainer"

# Install build dependencies
RUN dnf update -y && \
    dnf install -y git ca-certificates gcc gcc-c++ make golang && \
    dnf clean all && \
    rm -rf /var/cache/dnf

# Create non-root user
RUN groupadd -r -g 1000 msaki && useradd -r -g msaki -u 1000 msaki && \
    mkdir -p /etc/msaki /var/log/msaki && \
    chown -R msaki:msaki /etc/msaki /var/log/msaki && \
    chmod 755 /etc/msaki /var/log/msaki && \
    chown -R msaki:msaki /opt/app-root/

# Use non-root user
USER msaki

# Expose default port
EXPOSE 8080
