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
- OTel Tracing

Todo:

- Runtime Templates
- Model Explorer (NGC/HF Browser)
- [Punted] RBAC Enhancements
- [Punted] OIDC Integration
- [Punted] Conversation and model awareness

## How to Use

To build and run:

### Development (separate frontend/backend)

```bash
make dev-backend   # Start Go server on :8080
make dev-frontend  # Start NextJS on :3000
```

### Production (single binary embedded frontend)

```bash
make build         # Builds frontend, copies to web/static, builds Go binary
./bin/msaki -config configs/msaki.example.yaml
```

### As a Container

```bash
# Build the Container - or make container-build
docker build -t msaki -f Dockerfile -.

# Run the container - or make container-run
docker run --rm -it -d --name msaki -v ./configs/msaki.container-example.yaml:/etc/msaki/msaki.yaml -p 8080:8080 msaki
```

The example htpasswd users are `admin/admin123` and `user/user123`.
