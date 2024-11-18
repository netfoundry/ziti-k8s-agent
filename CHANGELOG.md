# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [0.1.1] - 2024-09-27

- Updated the security context of the sidecar container

  ```shell
  Capabilities: &corev1.Capabilities{
	Add:  []corev1.Capability{"NET_ADMIN"},
	Drop: []corev1.Capability{"ALL"},
  },
  RunAsUser:  &rootUser, (deafault = true)
  Privileged: &isPrivileged, (default = false)
  ```

## [0.1.0] - 2024-08-08

- Added initial code.

