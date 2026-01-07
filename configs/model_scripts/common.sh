#!/bin/bash

export ENV_CACHE_PATHS="-e HF_HOME=/vllm-cache/huggingface \
  -e XDG_CONFIG_HOME=/vllm-cache/vllm \
  -e XDG_CACHE_HOME=/vllm-cache \
  -e TORCHINDUCTOR_CACHE_DIR=/vllm-cache/torchinductor \
  -e TIKTOKEN_RS_CACHE_DIR=/vllm-cache/tiktoken \
  -e TIKTOKEN_ENCODINGS_BASE=/vllm-cache/tiktoken-encodings"

export ENV_HTTP_PROXY="-e http_proxy= -e https_proxy= -e no_proxy="

export CACHE_VOLUME_MOUNT="-v /opt/workdir/vllm-cache:/vllm-cache"