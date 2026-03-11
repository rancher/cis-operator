# cis-operator [![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/rancher/cis-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/rancher/cis-operator)

The cis-operator enables running CIS benchmark security scans on a Kubernetes cluster and generate compliance reports that can be downloaded. Benchmarks tests and the execution logic lives on [rancher/security-scan](https://github.com/rancher/security-scan).

> [!IMPORTANT]
> **This project has been replaced by [rancher/compliance-operator](https://github.com/rancher/compliance-operator).** 
> It is now in a **Security Maintenance Phase** for Rancher 2.10 and 2.11. No new features will be added.

---

## Migration
For Rancher v2.12.0+, the legacy CIS Benchmark App is replaced by the new **[Rancher Compliance App](https://ranchermanager.docs.rancher.com/how-to-guides/advanced-user-guides/compliance-scan-guides)**. Please follow the official guide to migrate your custom profiles:

👉 **[Migration Guide: CIS Benchmark to Rancher Compliance](https://support.scc.suse.com/s/kb/Migrating-from-Rancher-CIS-Benchmark-to-Rancher-Compliance?language=en_US)**

---

## Support Status

| Branch | Rancher Version | Status | Recommended Action |
| :--- | :--- | :--- | :--- |
| **release/v1.4** | v2.11.x | **Security Only** | Move to [Compliance Operator](https://github.com/rancher/compliance-operator) |
| **release/v1.3** | v2.10.x | **Security Only** | Plan migration for Rancher 2.12+ |
| **release/v1.2** | v2.9.x | **EOL** | Upgrade to supported version |

### Maintenance Policy
* **Issues & Feedback:** Direct all new feature requests or general bugs to the [Compliance Operator Issues](https://github.com/rancher/compliance-operator/issues).
* **Security Patches:** We will continue to review PRs for documented security vulnerabilities (CVEs) on supported 2.10/2.11 lines. All other PRs will be redirected.

---

## Branches and Releases
### General information
The current branch strategy for `rancher/cis-operator` is laid out below:

| Branch                | Tag      |Security-Scan          | Rancher                   |
|-----------------------|----------|-----------------------|---------------------------|
| `main`                | `head`   |`main` branch (`head`)`| `main` branch (`head`)    |
| `release/v1.3`        | `v1.3.x` |`v0.5.x`               | `v2.10.x`                 |
| `release/v1.2`        | `v1.2.x` |`v0.4.x`               | `v2.9.x`                  |
| `release/v1.1`        | `v1.1.x` |`v0.3.x`               | `v2.8.x`                  |
| `master` (deprecated) | `v1.0.x` |`v0.2.x`               | `v2.7.x`,`v2.8.x`,`v2.9.x`|

Note that it aligns with Rancher Manager releases to maximize compatibility
within the ecosystem. This includes k8s dependencies that the Rancher release
aims to support, meaning that cis-operator should use the same k8s minor release
that the Rancher release line it aims to support.

Active development takes place against `main`. Release branches are only used for
bug fixes and security-related dependency bumps.

Refer to the [Support Compatibility Matrix](https://www.suse.com/suse-rancher/support-matrix) for official compatibility information.

## License
Copyright (c) 2019 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

[rancher/security-scan]: https://github.com/rancher/security-scan
