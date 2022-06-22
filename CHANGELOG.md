# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

<!--
Please add your changelog entry under this comment in the correct category (Security, Fixed, Added, Changed, Deprecated, Removed - in this order).
-->

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
