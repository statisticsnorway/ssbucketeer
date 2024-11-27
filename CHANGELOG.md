# Changelog

## [0.0.45](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.44...v0.0.45) (2024-11-27)


### Bug Fixes

* remove configmap stuff from sts webhook ([76a362f](https://github.com/statisticsnorway/ssbucketeer/commit/76a362f797ae34be27bfc669f624691a93ec6a70))

## [0.0.44](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.43...v0.0.44) (2024-11-27)


### Bug Fixes

* move configmap mounting to job controller ([296888c](https://github.com/statisticsnorway/ssbucketeer/commit/296888c17d1a3ddc9641ed20ef1b1d084e2c6201))

## [0.0.43](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.42...v0.0.43) (2024-11-27)


### Bug Fixes

* grant create permission on configmaps ([2071e84](https://github.com/statisticsnorway/ssbucketeer/commit/2071e84547cdb2ebb33641c95360187ffa4f0dca))

## [0.0.42](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.41...v0.0.42) (2024-11-27)


### Features

* mount a script for refreshing buckets in statefulsets ([#69](https://github.com/statisticsnorway/ssbucketeer/issues/69)) ([a58e373](https://github.com/statisticsnorway/ssbucketeer/commit/a58e37377a4e369cc1fac0bfb7e44dcb2fa43cd2))

## [0.0.41](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.40...v0.0.41) (2024-11-22)


### Features

* add refresh-folder endpoint to precreator ([#68](https://github.com/statisticsnorway/ssbucketeer/issues/68)) ([ca080d2](https://github.com/statisticsnorway/ssbucketeer/commit/ca080d25481f09e661d2dc4b3bc0952483ebcc17))


### Bug Fixes

* Dockerfile.iam-probe to reduce vulnerabilities ([#65](https://github.com/statisticsnorway/ssbucketeer/issues/65)) ([04022dd](https://github.com/statisticsnorway/ssbucketeer/commit/04022dd8d208a8ec0b1afc111d4a500a0de93c8f))

## [0.0.40](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.39...v0.0.40) (2024-11-08)


### Bug Fixes

* trim trailing hyphen from team name ([5dcd4c9](https://github.com/statisticsnorway/ssbucketeer/commit/5dcd4c93f1b4493f41c4ccf3158d4fbfeead0451))

## [0.0.39](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.38...v0.0.39) (2024-11-08)


### Bug Fixes

* use base64 encoded json as id ([6f9c374](https://github.com/statisticsnorway/ssbucketeer/commit/6f9c374f680159efd2eb1014251a3cd20d1ff18c))

## [0.0.38](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.37...v0.0.38) (2024-11-08)


### Features

* unique cloud logging timestamp and ID ([11c73ba](https://github.com/statisticsnorway/ssbucketeer/commit/11c73ba999d321565755e3c0efeb1ed286721ca8))


### Bug Fixes

* revert logging only on creation ([87687c4](https://github.com/statisticsnorway/ssbucketeer/commit/87687c43914bcd76f72c48a309c783411c403bd9))

## [0.0.37](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.36...v0.0.37) (2024-11-08)


### Bug Fixes

* log human readable duration ([64f64be](https://github.com/statisticsnorway/ssbucketeer/commit/64f64bef765dac9d067ca036a72bdfbbb2f1652a))
* only log acccess on SA creation ([418414f](https://github.com/statisticsnorway/ssbucketeer/commit/418414f124289fd0cca0f42c0d2af65438610058))
* trim suffix to get team name ([35d5162](https://github.com/statisticsnorway/ssbucketeer/commit/35d5162b269a925835cc5a801ba0981e09f4b060))

## [0.0.36](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.35...v0.0.36) (2024-11-08)


### Bug Fixes

* propagate access reason to SA ([bbb7480](https://github.com/statisticsnorway/ssbucketeer/commit/bbb748016baa8b9e21ca175e457faf17a4878ae6))

## [0.0.35](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.34...v0.0.35) (2024-11-08)


### Bug Fixes

* add yaml struct tags to auditSink ([bfa8edf](https://github.com/statisticsnorway/ssbucketeer/commit/bfa8edf5dee1805a046fbef125f0697424192376))
* remove extraneous call to handleServiceAccount ([#55](https://github.com/statisticsnorway/ssbucketeer/issues/55)) ([9907aec](https://github.com/statisticsnorway/ssbucketeer/commit/9907aecc348c60e930c5c36a3392323a4b48029f))

## [0.0.34](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.33...v0.0.34) (2024-11-08)


### Features

* external audit logging ([#56](https://github.com/statisticsnorway/ssbucketeer/issues/56)) ([962f007](https://github.com/statisticsnorway/ssbucketeer/commit/962f0073d9b319e9492320b9c799629bb19efec3))

## [0.0.33](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.32...v0.0.33) (2024-11-07)


### Bug Fixes

* use creationTimestamp as starting point in IAM condition ([b2bf659](https://github.com/statisticsnorway/ssbucketeer/commit/b2bf659902806de3ed0794726ff03fe5defe9f0d))

## [0.0.32](https://github.com/statisticsnorway/ssbucketeer/compare/v0.0.31...v0.0.32) (2024-10-30)


### Bug Fixes

* define reason annotation as const string ([e27d729](https://github.com/statisticsnorway/ssbucketeer/commit/e27d7299f1f681f0a2ffeb452cf23e0dabf4399e))

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
