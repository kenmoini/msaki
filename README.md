# Model Swiss Army Knife Interface (MSAKI)

MSAKI is the answer to "How do I serve multiple models from a central place without needing to manage them and their lifecycle?"

MSAKI allows for easy proxying of requests to different OpenAI or Ollama compatible endpoints.  The endpoints can be defined as URI targets or start/stop scripts that can load/unload models run via command line execution of binaries or containers.

The main capabilities are:

- Configure different Runtimes and their defaults
- Proxies Model Requests to different OpenAI/Ollama compatible Endpoints
- Built-in Chat Interface
- Hot loading and unloading of models
- Run Tests against running models
- Save Chat History
- Prometheus Metrics
- Access and Error Logging

Todo:

- OTel Tracing
- OIDC Integration