# Claude Code Container mit persistentem OAuth-Login
FROM node:22-slim

# Grundlegende Tools, die Claude Code / typische Coding-Tasks brauchen
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    bash \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Claude Code CLI installieren
RUN npm install -g @anthropic-ai/claude-code

# Arbeitsverzeichnis für dein Projekt
WORKDIR /workspace

# Interaktive Shell als Standard, damit du claude manuell starten kannst
CMD ["bash"]
