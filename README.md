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
    image: ghcr.io/fogandfrog/pomerium:v0.34.1-build.2
    args: ["--config", "/pomerium/config.yaml"]
```

Supply your own configuration and secrets using normal Pomerium deployment practices. The image contains no application-specific routes, domains, credentials or deployment automation. Its default entrypoint is `/bin/pomerium`.

Versions use `<upstream-version>-build.<packaging-revision>`, for example `v0.34.1-build.2`. These revision tags are write-once. Pin production deployments to the **manifest digest** recorded in the successful build's summary, rather than to the `latest` convenience tag. Publishing a new image never deploys it to a cluster.

## Envoy admin connection leak backport

Build revision 4 backports the Envoy admin client cleanup from [upstream PR #7028](https://github.com/pomerium/pomerium/pull/7028) to v0.34.1. With Go 1.27, the readiness check's newly allocated HTTP transport retains an idle admin connection every 30 seconds. Over several days, the accumulated connections consume enough container memory to activate Envoy's overload protection. At 95% of the memory limit, Envoy stops accepting public connections while its readiness endpoint continues reporting healthy; the kernel does not OOM-kill the container because it remains below its hard limit.

The backport explicitly closes idle connections after readiness, dynamic extension status and drain requests. It preserves the existing memory limit and overload protection. The build runs a regression test against the actual readiness implementation with a chunked Envoy-style response, requiring all connections to close after repeated checks. Patch application refuses mismatched or already patched source; remove the backport deliberately when upgrading to an upstream release that includes the fix.

The 2026-11-23 Aritas incident confirmed 18,200 retained admin connections after six days, 95.76% cgroup memory saturation, active `stop_accepting_connections`, and zero cgroup OOM events. Restarting Pomerium restored the login redirect and reduced reported memory from 939 MiB to 61 MiB. Restarting is temporary recovery; deploy the backported image to prevent recurrence.

## Build, tests and security

The [Build workflow](.github/workflows/build.yaml) checks source identity and licensing, audits source dependencies before execution, builds custom Envoy, the UI and Core from source, and runs upstream version/analytics tests plus our no-reporting regression test. It verifies the Core license and the source-built Envoy license stamp, scans the final image without exclusions, confirms Go runtime/dependency detection and generates a CycloneDX SBOM. All third-party Actions are pinned to commit hashes.

The vulnerability gate blocks CISA KEV matches, EPSS scores of at least 0.10, and fixable HIGH/CRITICAL findings. Unfixed findings below those exploitation thresholds remain visible in the report; a passing gate does not mean zero vulnerabilities. A separate daily Security scan checks the published `latest` image against current advisories. Trivy's Go inventory does not establish complete coverage of the C++ executable embedded in Core; Envoy advisories require separate review.

Each build retains its version/license output, raw and enriched vulnerability reports, SBOM and published digest in the `pomerium-build-evidence` Actions artifact. Envoy module/origin metadata, evaluated Bazel dependencies and its license check are retained separately in `envoy-build-evidence`. Dependency license and notice files available in the source trees are bundled under `/usr/share/licenses/pomerium-build` in the image; upstream Core's license also remains at `/pomerium/LICENSE`.

## Automatic upstream release checks

A scheduled workflow runs **daily at 09:17 UTC**. It resolves the latest non-draft, non-prerelease Pomerium release to a commit, checks whether its packaging revision already exists, and builds/publishes only a new revision. It does not rebuild or overwrite an existing tag. Security rescanning runs daily at 09:41 UTC.

Automatic publication is conditional on the same licensing, compatibility, tests and vulnerability gates as a manual build. In particular, a release that changes the source license or uses an unsupported dependency declaration stops for maintenance review. The workflow will not silently substitute an old runtime for a new one or distribute a newly restricted binary. A stopped build is visible in Actions; it is not a successful upgrade.

To run manually, select **Build → Run workflow** on `main`, then choose `pinned` for the reviewed default or `latest` for the latest stable release. `config/source.env` records the default source and packaging revision. Increment `BUILD_REVISION` to publish packaging changes for an existing upstream version. Changes to the build pipeline go through a PR and static/behavioral checks.

## Telemetry defaults

Core is compiled with `officialBuild=0`, selecting its open-source license type. Upstream's usage reporter returns without sending for that type; our regression test exercises its send path even when the product-analytics flag is enabled. The image also defaults to:

```text
RUNTIME_FLAGS={"product_analytics":false,"routes_portal":false}
OTEL_TRACES_EXPORTER=none
DO_NOT_TRACK=1
```

The first two settings explicitly disable product analytics/the routes portal and trace export. `DO_NOT_TRACK` is an additional convention for versions supporting it; the no-reporting guarantee for the reviewed version does not depend on that variable. No Pomerium Zero or Enterprise Console connection is configured. Local logs and metrics remain available. Identity-provider and other application-required connections still occur normally.

## Envoy built from source

Core's default helper downloads a prebuilt custom Envoy binary. We bypass that download and compile the exact `pomerium/envoy-custom` source selected by the chosen Core release. Its source is Apache-2.0; our build explicitly selects the upstream `Apache-2.0` license stamp. No restricted official binary is used and no license marker is removed or rewritten. The older Apache-distributed binary used in `build.1` is no longer needed for new builds.

Version resolution uses Go's selected module graph and checksum-verified download metadata, including the full Git origin commit. It supports stable tags, pseudo-versions, and same-module version replacements without parsing version strings or depending on `go.mod` line layout. Local/foreign replacements, workspace modules outside the checkout, missing origin/checksums and inconsistent metadata fail closed. The module declaration and checksums remain unchanged.

Bazel evaluates custom Envoy's dependency declarations itself. Its structured query output records the exact upstream Envoy archive commit and SHA-256, so comments, variable names, whitespace and splitting declarations into `.bzl` files do not affect detection. A new repository rule, Bzlmod migration, unpinned archive or unrecognized URL stops for review. This is deliberately strict about changed semantics while tolerating formatting and layout changes.

The reusable [Envoy workflow](.github/workflows/envoy.yaml) compiles the normal dynamic-extension-capable `envoy.stripped` target, with the custom extensions selected upstream. Its intermediate image uses the same public package with an `envoy-<full-commit>-build.<revision>` tag. It contains the executable, dependency metadata and available license/notice files, and is a build input rather than a runnable service image. Unchanged custom Envoy commits reuse that immutable image, verified by digest, revision and license metadata. Increment `ENVOY_BUILD_REVISION` when changing Envoy build flags or packaging.

Core embeds that digest-pinned executable with a SHA-256 lockfile generated using upstream's own lockfile type and version calculation. The generator checks the ELF license stamp; the final version check must report `Apache-2.0` for Envoy. The C++ compilation is substantially heavier than Core's Go build, so the first build of an Envoy revision can take several hours on a hosted runner. Build or resource failures prevent publication of a new Core image.

## License

The build code in this repository is [Apache-2.0](LICENSE). Pomerium Core, custom Envoy and upstream Envoy retain their source licenses; dependencies retain their respective licenses and attribution requirements. See [NOTICE](NOTICE). Apache-2.0 grants no rights to Pomerium trademarks. These images are independent builds, not official Pomerium releases.
