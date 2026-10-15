# yamlfmt

`yamlfmt` is an extensible command line tool or library to format yaml files. 

## Goals

* Create a command line yaml formatting tool that is easy to distribute (single binary)
* Make it simple to extend with new custom formatters
* Enable alternative use as a library, providing a foundation for users to create a tool that meets specific needs 

## Maintainers

This tool is not yet officially supported by Google. It is currently maintained solely by @braydonk, and unless something changes primarily in spare time.

## Blog

I'm going to use these links to GitHub Discussions as a blog of sorts, until I can set up something more proper:
* yamlfmt's recent slow development [#149](https://github.com/google/yamlfmt/discussions/149)
* Issues related to the yaml.v3 library [#148](https://github.com/google/yamlfmt/discussions/148)

## Installation

To download the `yamlfmt` command, you can download the desired binary from releases or install the module directly:
```
go install github.com/forkcloser/yamlfmt/cmd/yamlfmt@latest
```
This currently requires Go version 1.21 or greater.

NOTE: Recommended setup if this is your first time installing Go would be in [this DigitalOcean blog post](https://www.digitalocean.com/community/tutorials/how-to-build-and-install-go-programs).

You can also download the binary you want from releases. The binary is self-sufficient with no dependencies, and can simply be put somewhere on your PATH and run with the command `yamlfmt`. Read more about verifying the authenticity of released artifacts [here](#verifying-release-artifacts).

You can also install the command as a [pre-commit](https://pre-commit.com/) hook. See the [pre-commit hook](./docs/pre-commit.md) docs for instructions.

## Basic Usage

See [Command Usage](./docs/command-usage.md) for in-depth information and available flags.

To run the tool with all default settings, run the command with a path argument:
```bash
yamlfmt x.yaml y.yaml <...>
```
You can specify as many paths as you want. You can also specify a directory which will be searched recursively for any files with the extension `.yaml` or `.yml`.
```bash
yamlfmt .
```

You can also use an alternate mode that will search paths with doublestar globs by supplying the `-dstar` flag. 
```bash
yamlfmt -dstar **/*.{yaml,yml}
```
See the [doublestar](https://github.com/bmatcuk/doublestar) package for more information on this format.

Yamlfmt can also be used in ci/cd pipelines which supports running containers. The following snippet shows an example job for GitLab CI:
```yaml
yamllint:
  image: ghcr.io/google/yamlfmt:latest
  script:
    - yamlfmt -lint .
```
The Docker image can also be used to run yamlfmt without installing it on your system. Just mount the directory you want to format as a volume (`/project` is used by default):
```bash
docker run -v "$(pwd):/project" ghcr.io/google/yamlfmt:latest <yamlfmt args>
```

# Configuration File

The `yamlfmt` command can be configured through a yaml file called `.yamlfmt`. This file can live in your working directory, a path specified through a [CLI flag](./docs/command-usage.md#operation-flags), or in the standard global config path on your system (see docs for specifics).
For in-depth configuration documentation see [Config](docs/config-file.md).

## Verifying release artifacts

Every release ships `checksums.txt`, listing each archive's SHA-256, and `checksums.txt.sigstore.json`, its cosign signature. The release workflow signs keyless: the signer is the workflow itself at the release's tag, so no key exists to leak. Steps to verify (replace `X.Y.Z` with the version you want):

1. Download the checksums, their signature and the archive you want:

   ```shell
   curl -sfLO https://github.com/forkcloser/yamlfmt/releases/download/vX.Y.Z/checksums.txt
   curl -sfLO https://github.com/forkcloser/yamlfmt/releases/download/vX.Y.Z/checksums.txt.sigstore.json
   curl -sfLO https://github.com/forkcloser/yamlfmt/releases/download/vX.Y.Z/yamlfmt_X.Y.Z_linux_amd64.tar.gz
   ```

2. Verify the signature against the release workflow's identity:

   ```shell
   cosign verify-blob checksums.txt \
     --bundle checksums.txt.sigstore.json \
     --certificate-identity https://github.com/forkcloser/yamlfmt/.github/workflows/release.yaml@refs/tags/vX.Y.Z \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com
   ```

3. Validate the archive's checksum, then unpack it:

   ```shell
   sha256sum --ignore-missing -c checksums.txt
   tar -xzf yamlfmt_X.Y.Z_linux_amd64.tar.gz
   ./yamlfmt -version
   ```
