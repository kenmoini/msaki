# Model Swiss Army Knife Interface (MSAKI)

MSAKI is a Golang based application that presents a CLI and a NextJS Web UI for managing different models.  It allows for easy proxying of requests to different OpenAI or Ollama compatible endpoints.  The endpoints can be defined as URI targets or start/stop scripts that can load/unload models run via command line execution of binaries or containers.

The main capabilities are:

- Configure different Runtimes and their defaults
- Proxies Model Requests to different OpenAI/Ollama compatible Endpoints
- Built-in Chat Interface
- Hot loading and unloading of models
- Run Tests against running models
- Save Chat History
- Prometheus Metrics
- Access and Error Logging

The APIs will be served with Golang's Gin framework and the Web UI will be presented with a NextJS SPA and Park UI implementation embedded into the created Golang binary.

Configuration will be provided via YAML similar to the following:

```yaml
global:
  # server provides the structure for configuring the MSAKI server
  server:
    # listen defines what address/port the MSAKI server binds to
    listen:
      address: 0.0.0.0
      port: 8080
    # portMapping defines how executed services should have a port mapped dynamically to it
    # If a ${PORT} string is found in a model's startScript then MSAKI will dynamically provide a port number for use to avoid conflicts
    portMapping:
      # containerLabelOverrides allows for a container to use a msaki.proxyPort label to specify what port to use instead of having MSAKI dynamically provide one
      containerLabelOverrides: true
      # hostPortStart is the range where MSAKI managed ports will start from
      hostPortStart: 12000
    # authentication provides a way to allow users to log in
    # Currently supports HTPasswd file authentication, to be expanded to OIDC/OAuth2 in the future
    authentication:
      - name: admin_userpass
        provider: htpasswd
        path: /path/to/htpasswd_file
        role: administrator
      - name: chat_users_userpass
        provider: htpasswd
        path: /path/to/other/htpasswd_file
        role: user
  observability:
    # Provides standard configuration for metrics endpoint
    metrics:
      enabled: true
      engine: prometheus
      prometheus:
        path: /metrics
    # accessLogs provide logging of proxy server requests
    accessLogs:
      enabled: true
      level: debug
      # sharedOutput if true will log to both file and stdout
      sharedOutput: true
      file: /var/logs/msaki/access.log
      fileRotation: 10Mb
    # errorLogs provide logging of proxy server errors
    errorLogs:
      enabled: true
      level: debug
      # sharedOutput if true will log to both file and stdout
      sharedOutput: true
      file: /var/logs/msaki/error.log
      fileRotation: 10Mb
    # chatLogs provide logging of chat messages
    chatLogs:
      enabled: true
      logDirectory: /var/log/msaki/chats/
      collections:
        - name: general
          models:
            - gpt-oss-120b
          requests: true
          responses: false
          filename: general-${MODEL}-${USERID}-${DATE-ymd}.log
        - name: exfil-detection
          models:
            - ext-openapi
            - ext-ollama
          requests: true
          responses: true
          filename: ext-${MODEL}-${USERID}-${DATE-ymd}.log

# models defines different endpoints that MSAKI proxies to.  Models listed here and detected by their APIs will be provided to consumers of MSAKI.
models:
  - name: gpt-oss-120b
    description: GPT OSS 120b via vLLM
    # aliases provides alternative or simple names for models
    aliases:
      - gpt-oss
    # tags are simple metadata to filter/group models
    tags:
      - general
      - chat
      - gpt
      - 120b
      - vllm
    # startScript runs when the model is request to be loaded
    # If ${PORT} is detected in the script then MSAKI will dynamically provide it a port to use
    startScript: docker run --rm --name gpt-oss-120b -p ${PORT}:8000 gpt-oss-120b-custom
    # stopScript if provided will allow the model to be stopped and unloaded
    stopScript: docker stop gpt-oss-120b
    # backendOverride provides an alternative endpoint to proxy requests to.  Useful for shell scripts or containers not exposing ports on an accessable network
    backendOverride: http://192.168.100.10:${PORT}
    # ttl if defined will auto stop and unload the model if there has not been activity sent to it
    ttl: 30m
    # healthCheck defines if MSAKI should detect availability of a model.
    # This is useful for models that take a long time to load
    healthCheck:
      enabled: true
      endpoint: /ping
      startDelay: 240s
      interval: 10s
      # retries if a non-zero number will attempt the healthcheck N number of times and finally stop the model if the healthcheck continues to fail.  Set to 0 to disable stopping of the model.
      retries: 12

  - name: ext-openapi
    description: External proxy to OpenAI
    aliases:
      - openai
      - chatgpt
    tags:
      - general
      - chat
      - gpt
      - external
    # endpoint provides configuration for an external model provider such as OpenAI
    endpoint: https://api.openai.com/v1
    # api_key_env specifies what environmental variable provides the API key.   Mutually exclusive with api_key_path
    api_key_env: OPENAI_API_KEY
    # api_key_path provides a way to load secrets from the file system.  Mutually exclusive with api_key_env
    api_key_path: /path/to/key
    healthCheck:
      endpoint: /health
      startDelay: 0s
      interval: 60s
      retries: 10

  - name: ext-ollama
    description: External Ollama server
    aliases:
      - ollama
    tags:
      - external
      - ollama
      - chat
    endpoint: https://remote-ollama:11434
    # skip_tls_verify allows for not validating TLS chains
    skip_tls_verify: true
    healthCheck:
      endpoint: /health
      startDelay: 0s
      interval: 60s
      retries: 10

tests:
  - name: Basic Chat Test
    description: Just a simple poem
    prompt: Write me a sea shanty about gouda cheese
    endpoint: /v1/chat/completions
    method: POST
  - name: Basic Coder Test
    description: Just a simple script
    prompt: Write me a Python script that finds duplicate files between two paths and gives the option to delete extra copies from one of them.
    endpoint: /v1/chat/completions
    method: POST
```
