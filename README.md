# Pomerium Build

[![Build](https://github.com/fogandfrog/pomerium-build/actions/workflows/build.yaml/badge.svg)](https://github.com/fogandfrog/pomerium-build/actions/workflows/build.yaml)
[![Checks](https://github.com/fogandfrog/pomerium-build/actions/workflows/checks.yaml/badge.svg)](https://github.com/fogandfrog/pomerium-build/actions/workflows/checks.yaml)
[![Security scan](https://github.com/fogandfrog/pomerium-build/actions/workflows/scan.yaml/badge.svg)](https://github.com/fogandfrog/pomerium-build/actions/workflows/scan.yaml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![GHCR](https://img.shields.io/badge/GHCR-fogandfrog%2Fpomerium-blue)](https://github.com/orgs/fogandfrog/packages/container/package/pomerium)
[![Upstream release](https://img.shields.io/github/v/release/pomerium/pomerium?label=upstream)](https://github.com/pomerium/pomerium/releases)

Independent Apache-licensed builds of [Pomerium Core](https://github.com/pomerium/pomerium), published as **`ghcr.io/fogandfrog/pomerium`** for reuse across projects. Maintained by [Fog&Frog](https://www.fogandfrog.com/). This project is not affiliated with or endorsed by Pomerium, Inc.

## Why this exists

Pomerium is useful infrastructure, and we want to keep using the upstream project across several applications without maintaining a separate build pipeline for every deployment. In November 2026, Pomerium [changed the license of its official builds](https://www.pomerium.com/blog/we-are-changing-how-we-license-pomerium-builds), while retaining Apache-2.0 for its source code. Official builds also gained usage reporting.

This repository provides an inspectable source build with usage reporting disabled, explicit dependency/license checks, vulnerability reports and a shared public container image. Applications keep their own routing, identity-provider configuration and secrets. This is a build repository rather than a full source fork: each build checks out an exact upstream commit and applies the small, documented packaging changes here.

## Use the image

The image is public and can be pulled from GHCR without credentials. The initial supported platform is **Linux amd64**.

```yaml
containers:
  - name: pomerium
    image: ghcr.io/fogandfrog/pomerium:v0.34.1-build.1
    args: ["--config", "/pomerium/config.yaml"]
```

Supply your own configuration and secrets using normal Pomerium deployment practices. The image contains no application-specific routes, domains, credentials or deployment automation. Its default entrypoint is `/bin/pomerium`.

Versions use `<upstream-version>-build.<packaging-revision>`, for example `v0.34.1-build.1`. These revision tags are write-once. Pin production deployments to the **manifest digest** recorded in the successful build's summary, rather than to the `latest` convenience tag. Publishing a new image never deploys it to a cluster.

## Build, tests and security

The [Build workflow](.github/workflows/build.yaml) checks source identity and licensing, audits source dependencies before execution, builds the UI and Core, and runs upstream version/analytics tests plus our no-reporting regression test. It verifies the Core license and the selected Envoy artifact, scans the final image without exclusions, confirms Go runtime/dependency detection and generates a CycloneDX SBOM. All third-party Actions are pinned to commit hashes.

The vulnerability gate blocks CISA KEV matches, EPSS scores of at least 0.10, and fixable HIGH/CRITICAL findings. Unfixed findings below those exploitation thresholds remain visible in the report; a passing gate does not mean zero vulnerabilities. A separate daily Security scan checks the published `latest` image against current advisories. Trivy's Go inventory does not establish complete coverage of the C++ executable embedded in Core; Envoy advisories require separate review.

Each build retains its version/license output, original Envoy OCI manifest, raw and enriched vulnerability reports, SBOM and published digest in the `pomerium-build-evidence` Actions artifact. Dependency license and notice files available in the source trees are bundled under `/usr/share/licenses/pomerium-build` in the image; upstream Core's license also remains at `/pomerium/LICENSE`.

## Automatic upstream release checks

A scheduled workflow runs **daily at 09:17 UTC**. It resolves the latest non-draft, non-prerelease Pomerium release to a commit, checks whether its packaging revision already exists, and builds/publishes only a new revision. It does not rebuild or overwrite an existing tag. Security rescanning runs daily at 09:41 UTC.

Automatic publication is conditional on the same licensing, compatibility, tests and vulnerability gates as a manual build. In particular, a release that changes the reviewed Envoy module or the source license stops for maintenance review. The workflow will not silently substitute an old runtime for a new one or distribute a newly restricted binary. A stopped build is visible in Actions; it is not a successful upgrade.

To run manually, select **Build → Run workflow** on `main`, then choose `pinned` for the reviewed default or `latest` for the latest stable release. `config/source.env` records the default source and packaging revision. Increment `BUILD_REVISION` to publish packaging changes for an existing upstream version. Changes to the build pipeline go through a PR and static/behavioral checks.

## Telemetry defaults

Core is compiled with `officialBuild=0`, selecting its open-source license type. Upstream's usage reporter returns without sending for that type; our regression test exercises its send path even when the product-analytics flag is enabled. The image also defaults to:

```text
RUNTIME_FLAGS={"product_analytics":false,"routes_portal":false}
OTEL_TRACES_EXPORTER=none
DO_NOT_TRACK=1
```

The first two settings explicitly disable product analytics/the routes portal and trace export. `DO_NOT_TRACK` is an additional convention for versions supporting it; the no-reporting guarantee for the reviewed version does not depend on that variable. No Pomerium Zero or Enterprise Console connection is configured. Local logs and metrics remain available. Identity-provider and other application-required connections still occur normally.

## The embedded Envoy licensing detail

Compiling Core alone is insufficient: its default helper currently fetches an Envoy artifact marked `LicenseRef-Pomerium-Community-Edition-1.0`. Our first build checks caught that distinction.

We select the original Apache-distributed artifact from `f7e948f7e6a4a55332f353ddb66103e9df3a7991`. The [comparison with upstream's selected revision](https://github.com/pomerium/envoy-custom/compare/f7e948f7e6a4a55332f353ddb66103e9df3a7991...e7b75c3a606d) changes only CI, licensing, OCI metadata and documentation; the Envoy 1.38.9 runtime/extension source is identical. The module's Go checksum-database hashes and the Linux amd64 manifest digest are recorded in `config/`.

The workflow checks the original manifest's `Apache-2.0` license annotation and the build verifies that the downloaded binary's lockfile references that exact digest. This older artifact predates the ELF license-stamp section, so `pomerium --version` prints `(no license found)` for Envoy. That output is accepted only for the exact independently verified artifact. We do not strip or rewrite license markers on restricted binaries.

Future Envoy runtime changes require a new license/compatibility review or our own source build of Envoy. The current pin must not become a reason to retain a vulnerable or incompatible runtime indefinitely.

## License

The build code in this repository is [Apache-2.0](LICENSE). Pomerium Core and the selected Envoy artifact retain their upstream licenses; dependencies retain their respective licenses and attribution requirements. See [NOTICE](NOTICE). Apache-2.0 grants no rights to Pomerium trademarks. These images are independent builds, not official Pomerium releases.
