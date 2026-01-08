#!/bin/bash

set -e

PORT=$1

# Change to the directory of the script
cd "$(dirname "$0")"

source ./common.sh

echo "----------------------------------------"
echo "Stopping and removing any existing container named vllm-qwen2.5-1.5B-instruct..."
docker stop vllm-qwen2.5-1.5B-instruct || true
docker rm vllm-qwen2.5-1.5B-instruct || true

# if [ ! -f "/vllm-cache/tiktoken-encodings/o200k_base.tiktoken" ]; then
#   echo "Downloading tiktoken o200k_base encodings..."
#   curl -o /vllm-cache/tiktoken-encodings/o200k_base.tiktoken "https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken"
# fi

# if [ ! -f "/vllm-cache/tiktoken-encodings/cl100k_base.tiktoken" ]; then
#   echo "Downloading tiktoken cl100k_base encodings..."
#   curl -o /vllm-cache/tiktoken-encodings/cl100k_base.tiktoken "https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken"
# fi

echo "----------------------------------------"
echo "Starting VLLM Qwen2.5-1.5B-Instruct on port ${PORT}..."
docker run --init --rm --name vllm-qwen2.5-1.5B-instruct \
  --runtime=nvidia --gpus all \
  --shm-size=1g --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  --network=proxy -p ${PORT}:8000 \
  ${ENV_HTTP_PROXY} \
  ${ENV_CACHE_PATHS} \
  ${CACHE_VOLUME_MOUNT} \
  -e HOME=/app \
  -e USER=app \
  nvidia-vllm:25.12-py3 \
  vllm serve \
  'Qwen/Qwen2.5-1.5B-Instruct' \
  --served-model-name 'Qwen2.5-1.5B-Instruct' \
  --gpu_memory_utilization 0.2