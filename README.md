# mobius
[![Go Report Card](https://goreportcard.com/badge/github.com/wjiec/mobius)](https://goreportcard.com/report/github.com/wjiec/mobius)
[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)


## Introduction

Mobius aims to better orchestrate services in a personal Homelab through kubernetes.


## Getting Started

This tutorial will detail how to configure and install the mobius to your cluster.

### Install mobius

If you have Helm, you can deploy the mobius with the following command:
```bash
helm upgrade --install mobius-manager mobius-manager \
    --repo https://wjiec.github.io/mobius \
    --namespace mobius-manager --create-namespace
```

It will install the mobius in the mobius-manager namespace, creating that namespace if it doesn't already exist.

### Configure a ExternalProxy

Create this manifests locally and update something to your own.
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
  service:
    type: ClusterIP
    ports:
    - name: http
      port: 80
  ingress:
    rules:
      - host: openwrt.home.lab
        http:
          paths:
            - pathType: ImplementationSpecific
              backend:
                port:
                  name: http
  tls:
    - hosts:
      - openwrt.home.lab
      secretName: star-home-lab
```


## Contributing

We warmly welcome your participation in the development of Mobius.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)


## License

Mobius is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
