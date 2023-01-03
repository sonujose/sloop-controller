## Development

1. Project Setup
```s

kubebuilder init --domain sloop.io --repo sloop.io/ctrl --plugins=go/v4-alpha

kubebuilder create api --group controller --version v1 --kind SloopController

```

2. Build and run Locally
```s
make
make manifests
make build
make install
make run ENABLE_WEBHOOKS=false
```

3. Apply the operator manifest
```s
kubectl apply -f prowconsole.yaml -n prow    
```

4. Build Operator Image and deploy

```s
IMAGE_TAG=1.0
IMAGE_REPO="us-west1-docker.pkg.dev/prow-open-btr/prow-dev-registry/prow-console-operator"

make docker-build docker-push IMG=${IMAGE_REPO}:${IMAGE_TAG}
make deploy IMG=${IMAGE_REPO}:${IMAGE_TAG}

```