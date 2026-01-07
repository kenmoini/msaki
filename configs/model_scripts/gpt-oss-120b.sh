#!/bin/bash

set -e

PORT=$1

source /app/model-scripts/common.sh

docker stop vllm-gpt-oss-120b || true
docker rm vllm-gpt-oss-120b || true

if [ ! -f "/vllm-cache/tiktoken-encodings/o200k_base.tiktoken" ]; then
  echo "Downloading tiktoken o200k_base encodings..."
  curl -o /vllm-cache/tiktoken-encodings/o200k_base.tiktoken "https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken"
fi

if [ ! -f "/vllm-cache/tiktoken-encodings/cl100k_base.tiktoken" ]; then
  echo "Downloading tiktoken cl100k_base encodings..."
  curl -o /vllm-cache/tiktoken-encodings/cl100k_base.tiktoken "https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken"
fi

docker run --init --rm --name vllm-gpt-oss-120b \
  --runtime=nvidia --gpus all \
  --shm-size=16g --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  --network=proxy -p ${PORT}:8000 \
  --user 1001:1001 \
  ${ENV_HTTP_PROXY} \
  ${ENV_CACHE_PATHS} \
  ${CACHE_VOLUME_MOUNT} \
  -e HOME=/app \
  -e USER=app \
  nvidia-vllm:25.12-py3 \
  vllm serve \
  'openai/gpt-oss-120b' \
  --served-model-name 'VLLM-GPT-OSS-120B' \
  --enable-expert-parallel \
  --load-format safetensors \
  --dtype auto \
  --swap-space 16 \
  --max-num-seqs 512 \
  --max-model-len 65536 \
  --gpu-memory-utilization 0.9 \
  --tensor-parallel-size 1