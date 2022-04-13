# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
* scripts for better developer experience @LittleFox94 (#48)

### Changes
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
