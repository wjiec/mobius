# mobius
[![Go Report Card](https://goreportcard.com/badge/github.com/wjiec/mobius)](https://goreportcard.com/report/github.com/wjiec/mobius)
[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)
[![CI](https://github.com/wjiec/mobius/actions/workflows/ci.yaml/badge.svg)](https://github.com/wjiec/mobius/actions/workflows/ci.yaml)


## Introduction

Mobius aims to better orchestrate services in a personal Homelab through kubernetes.

**Note: At the moment mobius is only aiming to serve my own Homelab needs and is in the
development phase, so the design and implementation logic will be changed very frequently.
If you are using the project or are interested in it, any suggestions or questions are welcome
(of course if you have a good idea and are also willing to provide PR, that's also very welcome!).**


## Getting Started

This tutorial will detail how to configure and install the mobius to your cluster.

### Install mobius

If you have Helm, you can deploy the mobius with the following command:
```bash
helm upgrade --install mobius-manager mobius-manager \
    --repo https://wjiec.github.io/mobius \
    --namespace mobius-system --create-namespace
```

It will install the mobius in the mobius-system namespace, creating that namespace if it doesn't already exist.

### ExternalProxy

As the name suggests, ExternalProxy creates a unified abstraction for out-of-cluster services through Kubernetes, allowing
us to create services or Ingresses for these out-of-cluster services as if they were in-cluster resources.

For example, I have some standalone services on my Homelab (TrueNas, openwrt) or something like that, and I'd like to
provide HTTPS ingress for those services via cert-manager or use names within the cluster to access specific services.

You can refer to the configuration below and update something to your own.
```yaml
apiVersion: networking.laboys.org/v1alpha1
kind: ExternalProxy
metadata:
  name: openwrt
spec:
  backends:
    - addresses:
        - ip: 172.16.1.1
      ports:
        - name: http
          port: 80
  ingress:
    rules:
      - host: openwrt.home.lab
  tls:
    - hosts:
        - openwrt.home.lab
      secretName: star-domain-com-tls
```


## Contributing

We warmly welcome your participation in the development of Mobius.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)


## License

Mobius is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
