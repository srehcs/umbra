FROM mcr.microsoft.com/devcontainers/base:ubuntu

RUN apt-get update && apt-get install -y       curl git ca-certificates build-essential       && rm -rf /var/lib/apt/lists/*

# Go
RUN curl -fsSL https://go.dev/dl/go1.22.6.linux-amd64.tar.gz -o /tmp/go.tgz       && rm -rf /usr/local/go       && tar -C /usr/local -xzf /tmp/go.tgz       && rm /tmp/go.tgz
ENV PATH="/usr/local/go/bin:${PATH}"

# Node + pnpm
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -       && apt-get update && apt-get install -y nodejs       && npm i -g pnpm

USER vscode
