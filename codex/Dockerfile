FROM ubuntu:24.04

ARG GO_VERSION=1.25.0

RUN apt-get update && apt-get install -y \
    make \
    curl \
    ca-certificates \
    git \
    jq \
    openssl \
    postgresql-client \
    redis-tools \
    dnsutils \
    netcat-openbsd \
    iputils-ping \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
       -o /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
       > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# yq — YAML processor for k8s manifest introspection.
ARG YQ_VERSION=v4.45.1
RUN ARCH=$(dpkg --print-architecture) \
    && curl -fsSL -o /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_${ARCH}" \
    && chmod +x /usr/local/bin/yq

# kustomize — render gitops overlays locally during debug runs.
ARG KUSTOMIZE_VERSION=v5.5.0
RUN ARCH=$(dpkg --print-architecture) \
    && curl -fsSL -o /tmp/kustomize.tgz "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_${ARCH}.tar.gz" \
    && tar -xzf /tmp/kustomize.tgz -C /usr/local/bin \
    && chmod +x /usr/local/bin/kustomize \
    && rm /tmp/kustomize.tgz

# Flux CLI — inspect HelmRelease / Kustomization / GitRepository state.
ARG FLUX_VERSION=2.4.0
RUN ARCH=$(dpkg --print-architecture) \
    && curl -fsSL -o /tmp/flux.tgz "https://github.com/fluxcd/flux2/releases/download/v${FLUX_VERSION}/flux_${FLUX_VERSION}_linux_${ARCH}.tar.gz" \
    && tar -xzf /tmp/flux.tgz -C /usr/local/bin \
    && chmod +x /usr/local/bin/flux \
    && rm /tmp/flux.tgz

# kubectl — pinned to a stable minor matching what the cluster runs.
# Tasks that need cluster introspection use this against the in-cluster
# API server via the mounted ServiceAccount token.
ARG KUBECTL_VERSION=v1.31.0
RUN ARCH=$(dpkg --print-architecture) \
    && curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl" \
    && curl -fsSL -o /tmp/kubectl.sha256 "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl.sha256" \
    && echo "$(cat /tmp/kubectl.sha256)  /usr/local/bin/kubectl" | sha256sum -c - \
    && chmod +x /usr/local/bin/kubectl \
    && rm /tmp/kubectl.sha256

# mise — required by platform-services' Makefile / pre-PR checks
# (mise run fmt:all / lint:all / build:all / test:all). Platform-services
# pins min_version=2025.2.6 in mise.toml; pin a recent stable release.
ARG MISE_VERSION=2025.5.0
RUN ARCH=$(uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/') \
    && curl -fsSL -o /tmp/mise.tgz "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/mise-v${MISE_VERSION}-linux-${ARCH}.tar.gz" \
    && tar -xzf /tmp/mise.tgz -C /tmp \
    && mv /tmp/mise/bin/mise /usr/local/bin/mise \
    && chmod +x /usr/local/bin/mise \
    && rm -rf /tmp/mise /tmp/mise.tgz

# pnpm — required by frontend pre-PR checks. Pinned to the version the
# frontend monorepo declares in packageManager (pnpm@10.27.0).
ARG PNPM_VERSION=10.27.0
RUN npm install -g pnpm@${PNPM_VERSION}

RUN ARCH=$(dpkg --print-architecture) \
    && TARBALL="go${GO_VERSION}.linux-${ARCH}.tar.gz" \
    && curl -fsSL -o "/tmp/${TARBALL}" "https://dl.google.com/go/${TARBALL}" \
    && curl -fsSL -o "/tmp/${TARBALL}.sha256" "https://dl.google.com/go/${TARBALL}.sha256" \
    && echo "$(cat /tmp/${TARBALL}.sha256)  /tmp/${TARBALL}" | sha256sum -c - \
    && tar -C /usr/local -xzf "/tmp/${TARBALL}" \
    && rm "/tmp/${TARBALL}" "/tmp/${TARBALL}.sha256"

ENV PATH="/usr/local/go/bin:${PATH}"

ARG CODEX_VERSION=0.130.0
RUN npm install -g @openai/codex@${CODEX_VERSION}

COPY codex/kelos_entrypoint.sh /kelos_entrypoint.sh
RUN chmod +x /kelos_entrypoint.sh

# Helper scripts for tasks that need git push + cluster read access. Each
# is a no-op when the corresponding env vars are unset, so adding them
# costs nothing for Q&A-style tasks that don't mount the github-app
# Secret or the ServiceAccount token. See codex/scripts/README for the
# env contract.
COPY codex/scripts/github-app-credential-helper /usr/local/bin/github-app-credential-helper
COPY codex/scripts/github-app-token /usr/local/bin/github-app-token
COPY codex/scripts/kelos-agent-setup /usr/local/bin/kelos-agent-setup
# gh wrapper sits at /usr/local/bin/gh (ahead of /usr/bin/gh in PATH) so
# every gh invocation picks up an App installation token automatically.
COPY codex/scripts/gh /usr/local/bin/gh
RUN chmod +x /usr/local/bin/github-app-credential-helper \
    /usr/local/bin/github-app-token \
    /usr/local/bin/kelos-agent-setup \
    /usr/local/bin/gh

# Alpheya engineering skills — baked at image build time so kelos-agent-setup
# can append selected SKILL.md files to ~/.codex/AGENTS.md without a per-task
# clone. Source repo (quantum-wealth/skills) is private, so the build needs a
# GitHub token via BuildKit secret: `docker buildx build --secret
# id=github_token,env=GITHUB_TOKEN ...`. When the secret is absent the build
# still succeeds (with an empty /opt/alpheya-skills) so a developer without
# the token can iterate locally; only Cody tasks that set ALPHEYA_SKILL_HOOKS
# require the directory populated.
ARG ALPHEYA_SKILLS_REPO=quantum-wealth/skills
ARG ALPHEYA_SKILLS_REF=main
RUN --mount=type=secret,id=github_token \
    if [ -s /run/secrets/github_token ]; then \
      TOKEN=$(cat /run/secrets/github_token) \
      && git clone --depth 1 --branch "${ALPHEYA_SKILLS_REF}" \
           "https://x-access-token:${TOKEN}@github.com/${ALPHEYA_SKILLS_REPO}.git" \
           /opt/alpheya-skills \
      && rm -rf /opt/alpheya-skills/.git ; \
    else \
      echo "WARNING: github_token build secret not provided; /opt/alpheya-skills will be empty" >&2 \
      && mkdir -p /opt/alpheya-skills ; \
    fi

ARG TARGETARCH
COPY bin/kelos-capture-linux-${TARGETARCH} /kelos/kelos-capture

RUN useradd -u 61100 -m -s /bin/bash agent
RUN mkdir -p /home/agent/.codex /home/agent/.local/bin && chown -R agent:agent /home/agent

USER agent

ENV GOPATH="/home/agent/go"
ENV PATH="/home/agent/.local/bin:${GOPATH}/bin:${PATH}"

WORKDIR /workspace

ENTRYPOINT ["codex"]
