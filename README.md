# ssbucketeer
A CRD-less, annotation-based controller which mutates StatefulSets and
ServiceAccounts so Google Cloud Storage buckets are mounted in the app using
gcsfuse.

## Getting Started

### Prerequisites
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/ssbucketeer:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/ssbucketeer:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/ssbucketeer:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/ssbucketeer/<tag or branch>/dist/install.yaml
```

# Contributing

Please follow these guidelines when contributing.

## Commit messages and merging PRs

Use squash merges, not merge commits.
This allows the release-please workflow to parse them and create a changelog.

This project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for its commit messages - **this also applies to squash merge messages**.
You can check out the following resources for more explanation/motivation:
[The power of conventional commits](https://julien.ponge.org/blog/the-power-of-conventional-commits/)
 and
[Conventional Commit Messages](https://gist.github.com/qoomon/5dfcdf8eec66a051ecd85625518cfd13).

When working on experimental branches you can use whatever commit messages you want, but you should either squash/amend your messages before merging your PR.
Using [Scratchpad branches](https://julien.ponge.org/blog/a-workflow-for-experiments-in-git-scratchpad-branches/) is probably the easiest approach.

Use the provided pre-commit hook to verify your commit messages:
```sh
pre-commit install --install-hooks
pre-commit install -t commit-msg
```

## Creating a release

Google's [release-please](https://github.com/googleapis/release-please) is used to create releases.
release-please maintains a release PR, which determines the next semver version based on whether there have been feature additions, breaking changes, etc.
To create a release, simply merge that PR, and it will create a GitHub release, tag and a Docker image will be built.

The suggested next version can be overriden by including `Release-As: x.x.x` in a commit message. For example:

```sh
git commit --allow-empty -m "chore: release 2.0.0" -m "Release-As: 2.0.0"
```
