#!/bin/bash

# Cria e usa um builder que suporta múltiplas plataformas
docker buildx create --use --name multiplatform-builder 2>/dev/null || docker buildx use multiplatform-builder

# Build para múltiplas plataformas (ARM64 e AMD64) e push
docker buildx build --platform linux/amd64,linux/arm64 -t evoapicloud/evolution-go:latest . --push