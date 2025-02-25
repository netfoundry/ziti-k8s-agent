# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2025-01-16

- https://github.com/netfoundry/ziti-k8s-agent/issues/31
- https://github.com/netfoundry/ziti-k8s-agent/issues/30
- https://github.com/netfoundry/ziti-k8s-agent/issues/28
- https://github.com/netfoundry/ziti-k8s-agent/issues/24
- https://github.com/netfoundry/ziti-k8s-agent/issues/14
- https://github.com/netfoundry/ziti-k8s-agent/issues/11

## [0.1.2] - 2024-11-22

- Added per pod injection to webhook

  ```shell
  objectSelector:
    matchLabels:
      openziti/ziti-tunnel: pod
  ```

## [0.1.1] - 2024-09-27

- Updated the security context of the sidecar container

  ```shell
  Capabilities: &corev1.Capabilities{
    Add:  []corev1.Capability{"NET_ADMIN", "NET_BIND_SERVICE"},
    Drop: []corev1.Capability{"ALL"},
  },
  RunAsUser:  &rootUser, (deafault = true)
  Privileged: &isPrivileged, (default = false)
  ```

## [0.1.0] - 2024-08-08

- Added initial code.
