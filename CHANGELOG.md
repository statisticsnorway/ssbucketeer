# Changelog

## [0.0.31](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.30...v0.0.31) (2024-10-30)


### Bug Fixes

* strip trailing hyphen from team name (for real) ([9ece0f3](https://github.com/statisticsnorway/ssbucketeer/commit/9ece0f3e37e57c34eea7dab6e00d4925fc16deec))

## [0.0.30](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.29...v0.0.30) (2024-10-30)


### Features

* modular project templating based on group config ([#48](https://github.com/statisticsnorway/ssbucketeer/issues/48)) ([256e996](https://github.com/statisticsnorway/ssbucketeer/commit/256e996b19f9895ed2372a0263b5c3437d2388e1))

## [0.0.29](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.28...v0.0.29) (2024-10-29)


### Features

* pass GroupConfigs to webhooks ([5a72b44](https://github.com/statisticsnorway/ssbucketeer/commit/5a72b44284ccb8fa6b494e95b835e04fc2d3581d))

## [0.0.28](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.27...v0.0.28) (2024-10-29)


### Features

* access duration restriction support ([270ac79](https://github.com/statisticsnorway/ssbucketeer/commit/270ac7919b8acff143c347ceaac16f9c4e1fc49c))


### Bug Fixes

* don't add ADC env var if it already exists ([f10cfcc](https://github.com/statisticsnorway/ssbucketeer/commit/f10cfcc00f6bd036112674505ff4060af87c609d))
* ignore NotFound error when deleting job ([bde2163](https://github.com/statisticsnorway/ssbucketeer/commit/bde2163d1cb4a52a986f106f8cbc57ee7507a17b))

## [0.0.27](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.26...v0.0.27) (2024-10-04)


### Features

* support mounting source data buckets ([#40](https://github.com/statisticsnorway/ssbucketeer/issues/40)) ([9a26a6c](https://github.com/statisticsnorway/ssbucketeer/commit/9a26a6c553d261a9036477e6667a4e038c2d1832))

## [0.0.26](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.25...v0.0.26) (2024-10-03)


### Bug Fixes

* remove panic-on-error in main ([#38](https://github.com/statisticsnorway/ssbucketeer/issues/38)) ([fab6846](https://github.com/statisticsnorway/ssbucketeer/commit/fab68466cdc0def0d2f66ab60ca976297d96c261))

## [0.0.25](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.24...v0.0.25) (2024-10-02)


### Bug Fixes

* **precreator:** add missing return ([e78c8aa](https://github.com/statisticsnorway/ssbucketeer/commit/e78c8aa18ecedd9fc293456b356cba806470ea2b))

## [0.0.24](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.23...v0.0.24) (2024-10-02)


### Bug Fixes

* **pre-creator:** ignore paths with storage-transfer prefix ([22e0679](https://github.com/statisticsnorway/ssbucketeer/commit/22e0679b841fb77b2ee3d0e33caca54460401712))

## [0.0.23](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.22...v0.0.23) (2024-08-30)


### Features

* Retry if the user name is kons ([#33](https://github.com/statisticsnorway/ssbucketeer/issues/33)) ([3e4a58a](https://github.com/statisticsnorway/ssbucketeer/commit/3e4a58a2549d01bf23fd33e4217c1164a90c97e8))

## [0.0.22](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.21...v0.0.22) (2024-08-16)


### Bug Fixes

* give jobs a time-to-live so their pods always get deleted ([#31](https://github.com/statisticsnorway/ssbucketeer/issues/31)) ([ae064f0](https://github.com/statisticsnorway/ssbucketeer/commit/ae064f070a6e30bc22d3b6e5e57e0663222fd4d8))

## [0.0.21](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.20...v0.0.21) (2024-08-16)


### Features

* inject a marker env var when a group is impersonated ([#29](https://github.com/statisticsnorway/ssbucketeer/issues/29)) ([5fc8d40](https://github.com/statisticsnorway/ssbucketeer/commit/5fc8d40c673d2ff47459ebab78487c6b643e9200))

## [0.0.20](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.19...v0.0.20) (2024-08-16)


### Bug Fixes

* skip precreator sidecar if no buckets are mounted ([#27](https://github.com/statisticsnorway/ssbucketeer/issues/27)) ([4a3271e](https://github.com/statisticsnorway/ssbucketeer/commit/4a3271e68f181ecf1865c0b3a5bfcbf2ca66111f))

## [0.0.19](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.18...v0.0.19) (2024-08-15)


### Bug Fixes

* update dependencies ([0500003](https://github.com/statisticsnorway/ssbucketeer/commit/05000032d4c4e471880dc37c4a36bc996277778a))

## [0.0.18](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.17...v0.0.18) (2024-08-15)


### Bug Fixes

* don't parse iamProbeStatus if it is done ([dd4f19f](https://github.com/statisticsnorway/ssbucketeer/commit/dd4f19feccf794e4413fcf88b58aa64346306dc0))

## [0.0.17](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.16...v0.0.17) (2024-08-14)


### Bug Fixes

* make precreator wait for kill signal ([db72701](https://github.com/statisticsnorway/ssbucketeer/commit/db727018af198aac5541a25aa205bd94c6c88d1c))

## [0.0.16](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.15...v0.0.16) (2024-08-14)


### Bug Fixes

* append to containers, not initContainers ([a325e24](https://github.com/statisticsnorway/ssbucketeer/commit/a325e248f7064eda200c0b1114c355663c10e34f))

## [0.0.15](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.14...v0.0.15) (2024-08-14)


### Bug Fixes

* start precreator as normal container ([b657e29](https://github.com/statisticsnorway/ssbucketeer/commit/b657e29ceee4b814088856827b326b8ab08db68b))

## [0.0.14](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.13...v0.0.14) (2024-08-14)


### Features

* folder precreator init container ([#20](https://github.com/statisticsnorway/ssbucketeer/issues/20)) ([572d94b](https://github.com/statisticsnorway/ssbucketeer/commit/572d94ba8cbd78b777ed16404c88539ef010dbb8))

## [0.0.13](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.12...v0.0.13) (2024-08-08)


### Bug Fixes

* pass iam probe image name to statefulset webhook ([b7f2c34](https://github.com/statisticsnorway/ssbucketeer/commit/b7f2c3483736fcd5f41b3d28535936be5a2c4ea1))

## [0.0.12](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.11...v0.0.12) (2024-08-08)


### Features

* add iam-probe dockerfile, change location to europe-west4 ([#17](https://github.com/statisticsnorway/ssbucketeer/issues/17)) ([1e8c637](https://github.com/statisticsnorway/ssbucketeer/commit/1e8c637cc2ce65307a2e53c9cc7127d79c2ff33d))

## [0.0.11](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.10...v0.0.11) (2024-08-08)


### Bug Fixes

* restore correct number of replicas ([#15](https://github.com/statisticsnorway/ssbucketeer/issues/15)) ([e842f6a](https://github.com/statisticsnorway/ssbucketeer/commit/e842f6a96a51b756f2d31f4f13a6d0ef2dc77046))

## [0.0.10](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.9...v0.0.10) (2024-08-07)


### Features

* suspend statefulset until iam probe job completes ([#13](https://github.com/statisticsnorway/ssbucketeer/issues/13)) ([5a39707](https://github.com/statisticsnorway/ssbucketeer/commit/5a397075486ceed34cc23e2115821f9f406da614))

## [0.0.9](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.8...v0.0.9) (2024-08-05)


### Bug Fixes

* use list API for team folder discovery ([#11](https://github.com/statisticsnorway/ssbucketeer/issues/11)) ([4856e95](https://github.com/statisticsnorway/ssbucketeer/commit/4856e95bde40e220e36876cd6c8e95d323c78601))

## [0.0.8](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.7...v0.0.8) (2024-08-05)


### Features

* use webhooks instead of reconcilers for validation ([#9](https://github.com/statisticsnorway/ssbucketeer/issues/9)) ([f065db3](https://github.com/statisticsnorway/ssbucketeer/commit/f065db3e66d70cb023abae6305c625d4d2e6757d))

## [0.0.7](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.6...v0.0.7) (2024-06-25)


### Bug Fixes

* actually pass the team name instead of an empty string ([846f080](https://github.com/statisticsnorway/ssbucketeer/commit/846f080c68f32e8362d8c944229eec1e5b0fc2f1))

## [0.0.6](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.5...v0.0.6) (2024-06-25)


### Bug Fixes

* try to fix iterator error, add some logging ([eaea2fe](https://github.com/statisticsnorway/ssbucketeer/commit/eaea2fec1119a5321b101fca4e80a324ac93071d))

## [0.0.5](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.4...v0.0.5) (2024-06-25)


### Bug Fixes

* create necessary clients fro sfs reconciler ([54f16d5](https://github.com/statisticsnorway/ssbucketeer/commit/54f16d5d15a46bc5bb0967143c6e38f207760ba1))

## [0.0.4](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.3...v0.0.4) (2024-06-25)


### Features

* support mounting standard buckets automatically ([bcf2b7e](https://github.com/statisticsnorway/ssbucketeer/commit/bcf2b7ebaca099c8d292f4023745271934392fe6))

## [0.0.3](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.2...v0.0.3) (2024-06-25)


### Features

* support custom mount points ([65de41a](https://github.com/statisticsnorway/ssbucketeer/commit/65de41aa3d3a0f0ac8e63e1424bae871e1413859))

## [0.0.2](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.1...v0.0.2) (2024-06-25)


### Bug Fixes

* remove explitic image tag ([6e60d40](https://github.com/statisticsnorway/ssbucketeer/commit/6e60d40311a48ecb4184eddafe0ac44bf618841e))

## 0.0.1 (2024-06-25)


### Features

* dummy release feat ([d39de25](https://github.com/statisticsnorway/ssbucketeer/commit/d39de2563c2e7e7b5d69b842db9353086fa7f1cd))
