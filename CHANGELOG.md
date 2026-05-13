# Changelog

## [2.0.0](https://github.com/rhysmcneill/ssmctl/compare/v1.0.1...v2.0.0) (2026-05-13)


### ⚠ BREAKING CHANGES

* v2.0.0 marks the completion of the Road to v2 milestone.

### Features

* **argument:** add --output argument for run, cp and version commands which allows users to return text (default) or json ([#61](https://github.com/rhysmcneill/ssmctl/issues/61)) ([39ea13f](https://github.com/rhysmcneill/ssmctl/commit/39ea13fbd637f4cf2d05ff3e0a8b4470f9b83ad7))
* **cmd:** add ssmctl list command with filtering arguments for better search capabilities ([#54](https://github.com/rhysmcneill/ssmctl/issues/54)) ([69f0017](https://github.com/rhysmcneill/ssmctl/commit/69f00176c633970c1d5cf3bfa22eaa22800a0e69))
* **cp:** add s3 file transfer option for large files ([#63](https://github.com/rhysmcneill/ssmctl/issues/63)) ([0ec2ad5](https://github.com/rhysmcneill/ssmctl/commit/0ec2ad5f6be940251112704dd187fcf5ebf00e62))
* **forward:** add ssmctl forward to allow users to port forward to an instance or remote host e.g. RDS ([#66](https://github.com/rhysmcneill/ssmctl/issues/66)) ([31d7bdf](https://github.com/rhysmcneill/ssmctl/commit/31d7bdfe7132813ee886bf615e1c32859c0eca53))
* release v2.0.0 ([#78](https://github.com/rhysmcneill/ssmctl/issues/78)) ([c57b23c](https://github.com/rhysmcneill/ssmctl/commit/c57b23c3e0c67737356f891b9052535acd88d869))
* **shell:** integrate shell completion for all shell types (fish, bash, zsh ([#74](https://github.com/rhysmcneill/ssmctl/issues/74)) ([efd5a76](https://github.com/rhysmcneill/ssmctl/commit/efd5a76ef4dfa735be08832cf3e89147d77e6ff6))
* **ssm:** handle unsupported Windows targets ([#42](https://github.com/rhysmcneill/ssmctl/issues/42)) ([c7158d6](https://github.com/rhysmcneill/ssmctl/commit/c7158d6b445b96113f2681623fa033fad471d51b))
* **windows:** add e2e support for windows ([#77](https://github.com/rhysmcneill/ssmctl/issues/77)) ([6796cba](https://github.com/rhysmcneill/ssmctl/commit/6796cba570082eb6ed4e0b925c1f9840006d57e3))


### Bug Fixes

* **app:** refactor sentinel ExitCodeError and implement interface-driven App clients ([#45](https://github.com/rhysmcneill/ssmctl/issues/45)) ([a4a6ea2](https://github.com/rhysmcneill/ssmctl/commit/a4a6ea21b4bcf8a8b69860028744093b87e41336))
* **client:** resolve an issue where by targets share the same name tag in AWS ([#52](https://github.com/rhysmcneill/ssmctl/issues/52)) ([8d46b41](https://github.com/rhysmcneill/ssmctl/commit/8d46b4171b6caec515f1613225d2a3d4c5ae4864))
* **docs:** update docs, code hygiene and formatting code. ([#26](https://github.com/rhysmcneill/ssmctl/issues/26)) ([e21949d](https://github.com/rhysmcneill/ssmctl/commit/e21949db836b192a76c867d5804ce59c79f2d53c))
* **internal:** wired debug flag ([#60](https://github.com/rhysmcneill/ssmctl/issues/60)) ([ec32de0](https://github.com/rhysmcneill/ssmctl/commit/ec32de024e8f34a379cbf65027fd8034b764059c))
* **transfer:** Implement heredoc for chunking file transfers ([#43](https://github.com/rhysmcneill/ssmctl/issues/43)) ([155d2f9](https://github.com/rhysmcneill/ssmctl/commit/155d2f902e5de06a9e17a0fa7e6562fdb58d5a4f))

## [1.0.1](https://github.com/rhysmcneill/ssmctl/compare/v1.0.0...v1.0.1) (2026-04-22)


### Bug Fixes

* **cmd:** solve evaluation on AWS_REGION, AWS_DEFAULT_REGION and ~/.aws/config ([#5](https://github.com/rhysmcneill/ssmctl/issues/5)) ([3d6dc66](https://github.com/rhysmcneill/ssmctl/commit/3d6dc669948cd6a326c19966aa16775d67196dd5))

## [1.0.0](https://github.com/rhysmcneill/ssmctl/compare/v0.1.0...v1.0.0) (2026-04-21)


### ⚠ BREAKING CHANGES

* **v1:** add roapmap main feature set. Implement testing, homebrew tap and install. Update docs. ([#2](https://github.com/rhysmcneill/ssmctl/issues/2))

### Features

* **v1:** add roapmap main feature set. Implement testing, homebrew tap and install. Update docs. ([#2](https://github.com/rhysmcneill/ssmctl/issues/2)) ([5359219](https://github.com/rhysmcneill/ssmctl/commit/5359219e39e671ade4f6145135a1fcd1fe3d0391))


### Bug Fixes

* **tests:** solve linter and e2e ci test bugs ([#4](https://github.com/rhysmcneill/ssmctl/issues/4)) ([4fdd7ea](https://github.com/rhysmcneill/ssmctl/commit/4fdd7eae8e2da03a816f2e18d1fcd97a1b6bbb96))

## [0.1.0](https://github.com/rhysmcneill/ssmctl/compare/v0.0.1...v0.1.0) (2026-04-19)


### Features

* **ssm:** integrate commands for running adhoc commnands on instance, transferring files, connecting to remote instance and add release-please for automated semver releases ([753d99f](https://github.com/rhysmcneill/ssmctl/commit/753d99fcd1b20c228adbb611b8bd66f8c47c2c12))
