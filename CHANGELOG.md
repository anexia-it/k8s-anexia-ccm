# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

<!--
Please add your changelog entry under this comment in the correct category (Security, Fixed, Added, Changed, Deprecated, Removed - in this order).
-->

### Changed

* various development dependency updates
  - Bump github.com/onsi/ginkgo/v2 from 2.1.3 to 2.8.4
  - Bump github.com/golangci/golangci-lint from 1.45.2 to 1.51.2

## [1.5.2] - 2023-01-23

### Changed
* instances: panic on 401 responses from engine to slow down requests with invalid token @marioreggiori (#155)
* various dependency updates
  - Bump github.com/onsi/ginkgo/v2 from 2.6.1 to 2.7.0 @dependabot (#153)
  - Bump github.com/onsi/gomega from 1.24.2 to 1.25.0 @dependabot (#154)
  - Bump k8s.io/cloud-provider from 0.26.0 to 0.26.1 @dependabot (#157)

### Removed
* (internal) Remove CVE-2022-27664 from trivyignore @LittleFox94 (#156)

## [1.5.1] - 2023-01-04

### Fixed
* fix LBaaS VIP discovery fallback @marioreggiori (#151)

### Changes
* various dependency updates
  - Bump go.anx.io/go-anxcloud from 0.4.6 to 0.5.0 @dependabot (#145)
  - Bump k8s.io/cloud-provider from 0.25.4 to 0.26.0 @dependabot (#147)
  - Bump github.com/onsi/ginkgo/v2 from 2.5.1 to 2.6.0 @dependabot (#148)
  - Bump github.com/onsi/ginkgo/v2 from 2.6.0 to 2.6.1 @dependabot (#149)
  - Bump github.com/onsi/gomega from 1.24.1 to 1.24.2 @dependabot (#150)

## [1.5.0] - 2022-11-29

### Added
* Use auto discovery for VIP addresses in auto discovered prefixes
  - Engine Addresses tagged with "kubernetes-lb-vip-<cluster name>" will be allocated for Services
  - If no tagged Address can be found, fall back to calculating the VIP. This fallback will be removed in the future
  - Configured prefixes always use calculated VIPs

### Changes
* (internal) refactor tests for LoadBalancer reconciliation
* various dependency updates
  - Bump go.anx.io/go-anxcloud from 0.4.5 to 0.4.6 @dependabot (#126)
  - Bump k8s.io/cloud-provider from 0.25.1 to 0.25.4 @dependabot (#125, #132, #141)
  - Bump github.com/prometheus/client\_golang from 1.13.0 to 1.14.0 @dependabot (#136, #138)
  - Bump github.com/stretchr/testify from 1.8.0 to 1.8.1 @dependabot (#133)
  - Bump github.com/onsi/ginkgo/v2 from 2.1.6 to 2.5.1 @dependabot (#124, #129, #131, #134, #139, #143)
  - Bump github.com/onsi/gomega from 1.20.2 to 1.24.1 @dependabot (#127, #130, #135, #137, #140)

## [1.4.4] - 2022-09-19

### Fixed
* Properly handle uninitialized optional features @marioreggiori (#120)

### Changes
* k8s-anexia-ccm is now built with Go 1.19 @LittleFox94 (#111)
* various dependency updates
  - Bump k8s.io/cloud-provider from 0.24.3 to 0.25.1 @dependabot (#112, #113, #121)
  - Bump k8s.io/klog/v2 from 2.70.1 to 2.80.1 @dependabot (#118, #119)
  - Bump github.com/onsi/gomega from 1.20.0 to 1.20.2 @dependabot (#114, #117)
  - Bump github.com/onsi/ginkgo/v2 from 2.1.4 to 2.1.6 @dependabot (#115, #116)
  - Bump go.anx.io/go-anxcloud from 0.4.4 to 0.4.5 @dependabot (#110)
  - Bump github.com/prometheus/client\_golang from 1.12.2 to 1.13.0 @dependabot (#109)

## [1.4.3]

### Fixes
* fix VM name prefix autodiscovery @LittleFox94 (#107)
  - by removing it alltogether and completely reworking the logic - see the PR for more details

### Changes
* Bump github.com/onsi/gomega from 1.19.0 to 1.20.0 @dependabot (#106)
* Bump k8s.io/cloud-provider from 0.24.2 to 0.24.3 @dependabot (#104)
* Bump k8s.io/klog/v2 from 2.60.1 to 2.70.1 @dependabot (#99, #103)
* Bump github.com/stretchr/testify from 1.7.4 to 1.8.0 @dependabot (#100, #102)

## [1.4.2] - 2022-06-22

### Fixes
* wrong usage of pointer-to-loop variable @LittleFox94 (#91)
  - definitely leading to bad performance when deleting Objects
  - might lead to wrongly created resources
* fix missed project-rename things, resulting in e.g. wrong version printed on startup @LittleFox94 (#88)

### Changes
* upgrade to Go 1.18 @LittleFox94 (#94)
* handle already existing LBaaS resources that are still progressing @LittleFox94 (#93)
* build and deploy docs to GitHub Pages at https://anexia-it.github.io/k8s-anexia-ccm @LittleFox94 (#89)

## [1.4.1] - 2022-05-04

### Fixes
* fix HealthCheck attribute on LBaaS Backend resources @LittleFox94 (#75)

### Changes
* upgrade go-anxloud to v0.4.3


## [1.4.0] - 2022-05-02

This release was made by accident, clicking "Publish Release" instead of "Save Draft" after tweaking the release notes.
Still nothing worse than in releases before.

### Added
* scripts for better developer experience @LittleFox94 (#48)

### Changes
* near-complete rewrite of LoadBalancer/LBaaS reconciliation @LittleFox94 (#46)
  - no more reconciling one LoadBalancer and syncing that to other LoadBalancers, just reconcile all of them at once
  - now a reconciliation loop similar to those in Kubernetes
  - reacting more graceful to error responses
* some preparations to OpenSource this project @LittleFox94 (#61)
* activated Dependabot @LittleFox94 (#54)
* upgraded some dependencies
  - github.com/stretchr/testify from 1.7.0 to 1.7.1 (#58)
  - k8s.io/klog/v2 from 2.30.0 to 2.60.1 (#51)
  - github.com/prometheus/client\_golang from 1.11.0 to 1.12.1 (#50)
  - github.com/go-logr/logr from 1.2.0 to 1.2.3 (#49)


## [1.3.0] - 2022-03-04

### Added
* feat(lbaas): Await LBaaS resources to be created or deleted @kstiehl (#44)
* ✨📝: add annotation for external IP families @LittleFox94 (#43)

### Changes
* Update go-anxcloud version to 0.4.1 @LittleFox94 (#45)


## [1.2.1] - 2022-02-24

### Fixes
* fix: ccm not listening to health and ready endpoints anymore @kstiehl (#42)


## [1.2.0] - 2022-02-23

### Added
* feat(metrics): add metrics for the anexia provider @kstiehl (#37)
* ✨🎨: add config for LB prefixes, autodiscover them @LittleFox94 (#36)

### Changes
* feat(documentation): update documentation to contain latest configuration options @kstiehl (#38)

### Fixes
* 💩✨🐛: store correct LoadBalancer IP in service (SYSENG-922) @LittleFox94 (#40)
* ♻️🔊: Replace panics by log and return (SYSENG-964) @kstiehl (#39)


## [1.1.3] - 2022-02-09

### Changes
* feat(lbaas-sync): improve lbaas configuration sync speed @kstiehl (#35)

### Fixes
* fix(lbaas): fix lbaas locking and fix cleanup @kstiehl (#34)


## [1.1.2] - 2022-02-08

### Fixes
* fix(lbaas): remove debug logs @kstiehl (#33)


## [1.1.1] - 2022-02-08

## Fixes
* Fix anx/provider/sync crash and possible deadlock @LittleFox94 (#32)


## [1.1.0] - 2022-02-07

### Added
* Add LoadBalancer Replication controller @kstiehl (#30)

### Fixes
* Consider CLI flags when parsing provider configuration @kstiehl (#27)
* fix(lbaas): Fix lbaas reconciliation @kstiehl (#31)
* Fix naming of resources @kstiehl (#26)

### Changes
* Update go-anxcloud version to 0.4.0 @kstiehl (#25)


## [1.0.0] - 2021-11-19

### Added
* Add Loadbalancer support to CCM @kstiehl (#24)

### Changed
* Update Go Version to 1.17 @kstiehl (#22)


## [0.1.0] - 2021-08-19

### Added
* feat(nodeController): Add intelligent node name resolving @kstiehl (#15)
* Add cloudprovider configuration documentation @kstiehl (#16)


## [0.0.1] - 2021-08-04

### Changes
* Implement Node Controller
