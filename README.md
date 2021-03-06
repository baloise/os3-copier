# os3-copier
OpenShift 3.x Version of the [k8s-copier](https://github.com/dweber019/k8s-copier)

This operator aims to provide Kubernetes CRD's to copy resources from one namespace into another.  
This new CRD is called `CopyResrouce` and the resulting resource is called target resource.

## Implemented resource types
- v1.Secret
- v1.ConfigMap

## Usage
You can find some usage examples in `config/samples/**`.

### Configuration
| Name                    | Type    | Default |
| ------------------------|---------|---------|
| WATCH_NAMESPACE         | env var | NA      |
| SYNC_PERIOD             | env var | 300     |
| metrics-addr            | flag    | :8080   |
| enable-leader-election  | flag    | false   |
| dev-mode-enabled        | flag    | false   |

### Permissions
You need a service account to operate your operator. This service account needs to have
access to the target namespace regarding the resource types.  
You can find examples in `config/samples/**`.

### Behavior
If you delete a CopyResource the target resource won't be deleted as it's possible that other implementation depend on it.

## Development setup
### Conventional commits
Execute the following terminal command in the root:
```
curl -o- https://raw.githubusercontent.com/craicoverflow/sailr/master/scripts/install.sh | bash
```
### Go
Follow the installation guide at https://golang.org/doc/install

### Operator-SDK
Uses the [Operator-SDK Version 0.18.2](https://github.com/operator-framework/operator-sdk/blob/v0.18.2) to be able to create and use CRDs compatible with OpenShift 3.x

## Development
For development, you should use `minishift` or any other possible kubernetes compatible implementation, as advised by the operator-sdk framework.

### Update the CRDs
To update the CRDs use the following command
```
make generate
make manifests
```
The files will be generated into `config/**`.

### Build and run
To install the CRDs to the Kubernetes cluster and run the operator outside of Kubernetes use
```
make install run
```

## Deployment
### Automated
The deployment and therefore publishing a Docker image is fully automated with GitHub workflows.

### Manually
Use the automated way over GitHub!!!
To update the docker image use this command
```
make docker-build docker-push IMG=docker.io/baloise/os3-copier:v0.0.1
```
After this you can run with
```
make deploy IMG=docker.io/baloise/os3-copier:v0.0.1
```

## Useful links
- [Kubebilder](https://book.kubebuilder.io)
- [Operator tutorial](https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/)
- [Conventional commits](https://www.conventionalcommits.org/en/v1.0.0/)