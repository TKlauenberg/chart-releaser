# Chart Releaser

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
![CI](https://github.com/helm/chart-releaser/workflows/CI/badge.svg?branch=main&event=push)

**Helps Turn GitHub Repositories into Helm Chart Repositories**

`cr` is a tool designed to help GitHub repos self-host their own chart repos by adding Helm chart artifacts to GitHub Releases named for the chart version and then creating an `index.yaml` file for those releases that can be hosted on GitHub Pages (or elsewhere!).

At the moment I test changes to the original cr tool in order to use cr with the github pages out of the actions result. Basically I don't want to use the gh-pages branch and not save the index.yaml file in my repository.

## Installation

### Binaries (recommended)

Download your preferred asset from the [releases page](https://github.com/helm/chart-releaser/releases) and install manually.

### Homebrew

```console
$ brew tap helm/tap
$ brew install chart-releaser
```

### Go get (for contributing)

```console
$ # clone repo to some directory outside GOPATH
$ git clone https://github.com/helm/chart-releaser
$ cd chart-releaser
$ go mod download
$ go install ./...
```

### Docker (for Continuous Integration)

Docker images are pushed to the [helmpack/chart-releaser](https://quay.io/repository/helmpack/chart-releaser?tab=tags) Quay container registry. The Docker image is built on top of Alpine and its default entry-point is `cr`. See the [Dockerfile](./Dockerfile) for more details.

## Usage

Currently, `cr` can create GitHub Releases from a set of charts packaged up into a directory and create an `index.yaml` file for the chart repository from GitHub Releases.

```console
$ cr --help
Create Helm chart repositories on GitHub Pages by uploading Chart packages
and Chart metadata to GitHub Releases and creating a suitable index file

Usage:
  cr [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  help        Help about any command
  index       Update Helm repo index.yaml for the given GitHub repo
  package     Package Helm charts
  upload      Upload Helm chart packages to GitHub Releases
  version     Print version information

Flags:
      --config string   Config file (default is $HOME/.cr.yaml)
  -h, --help            help for cr

Use "cr [command] --help" for more information about a command.
```

### Create GitHub Releases from Helm Chart Packages

Scans a path for Helm chart packages and creates releases in the specified GitHub repo uploading the packages.

```console
$ cr upload --help
Upload Helm chart packages to GitHub Releases

Usage:
  cr upload [flags]

Flags:
  -c, --commit string                  Target commit for release
      --generate-release-notes         Whether to automatically generate the name and body for this release. See https://docs.github.com/en/rest/releases/releases
  -b, --git-base-url string            GitHub Base URL (only needed for private GitHub) (default "https://api.github.com/")
  -r, --git-repo string                GitHub repository
  -u, --git-upload-url string          GitHub Upload URL (only needed for private GitHub) (default "https://uploads.github.com/")
  -h, --help                           help for upload
  -o, --owner string                   GitHub username or organization
  -p, --package-path string            Path to directory with chart packages (default ".cr-release-packages")
      --release-name-template string   Go template for computing release names, using chart metadata (default "{{ .Name }}-{{ .Version }}")
      --release-notes-file string      Markdown file with chart release notes. If it is set to empty string, or the file is not found, the chart description will be used instead. The file is read from the chart package
      --skip-existing                  Skip upload if release exists
  -t, --token string                   GitHub Auth Token
      --make-release-latest bool       Mark the created GitHub release as 'latest' (default "true")

Global Flags:
      --config string   Config file (default is $HOME/.cr.yaml)
```

### Create the Repository Index from GitHub Releases

Once uploaded you can create an `index.yaml` file that can be hosted on GitHub Pages (or elsewhere).

```console
$ cr index --help
Update a Helm chart repository index.yaml file based on a the
given GitHub repository's releases.

Usage:
  cr index [flags]

Flags:
  -b, --git-base-url string            GitHub Base URL (only needed for private GitHub) (default "https://api.github.com/")
  -r, --git-repo string                GitHub repository
  -u, --git-upload-url string          GitHub Upload URL (only needed for private GitHub) (default "https://uploads.github.com/")
  -h, --help                           help for index
  -i, --index-path string              Path to index file (default ".cr-index/index.yaml")
  -o, --owner string                   GitHub username or organization
  -p, --package-path string            Path to directory with chart packages (default ".cr-release-packages")
      --pages-branch string            The GitHub pages branch (default "gh-pages")
      --pages-index-path string        The GitHub pages index path (default "index.yaml")
      --pr                             Create a pull request for index.yaml against the GitHub Pages branch (must not be set if --push is set)
      --push                           Push index.yaml to the GitHub Pages branch (must not be set if --pr is set)
      --release-name-template string   Go template for computing release names, using chart metadata (default "{{ .Name }}-{{ .Version }}")
      --remote string                  The Git remote used when creating a local worktree for the GitHub Pages branch (default "origin")
  -t, --token string                   GitHub Auth Token (only needed for private repos)

Global Flags:
      --config string   Config file (default is $HOME/.cr.yaml)
```

## Configuration

`cr` is a command-line application.
All command-line flags can also be set via environment variables or config file.
Environment variables must be prefixed with `CR_`.
Underscores must be used instead of hyphens.

CLI flags, environment variables, and a config file can be mixed.
The following order of precedence applies:

1. CLI flags
1. Environment variables
1. Config file

### Examples

The following example show various ways of configuring the same thing:

#### CLI

    cr upload --owner myaccount --git-repo helm-charts --package-path .deploy --token 123456789

#### Environment Variables

    export CR_OWNER=myaccount
    export CR_GIT_REPO=helm-charts
    export CR_PACKAGE_PATH=.deploy
    export CR_TOKEN="123456789"
    export CR_GIT_BASE_URL="https://api.github.com/"
    export CR_GIT_UPLOAD_URL="https://uploads.github.com/"
    export CR_SKIP_EXISTING=true

    cr upload

#### Config File

`config.yaml`:

```yaml
owner: myaccount
git-repo: helm-charts
package-path: .deploy
token: 123456789
git-base-url: https://api.github.com/
git-upload-url: https://uploads.github.com/
```

#### Config Usage

    cr upload --config config.yaml


`cr` supports any format [Viper](https://github.com/spf13/viper) can read, i. e. JSON, TOML, YAML, HCL, and Java properties files.

Notice that if no config file is specified, `cr.yaml` (or any of the supported formats) is loaded from the current directory, `$HOME/.cr`, or `/etc/cr`, in that order, if found.

#### Notes for Github Enterprise Users

For Github Enterprise, `chart-releaser` users need to set `git-base-url` and `git-upload-url` correctly, but the correct values are not always obvious to endusers.

By default they are often along these lines:

```
https://ghe.example.com/api/v3/
https://ghe.example.com/api/uploads/
```

If you are trying to figure out what your `upload_url` is try to use a curl command like this:
`curl -u username:token https://example.com/api/v3/repos/org/repo/releases`
and then look for `upload_url`. You need the part of the URL that appears before `repos/` in the path.

##### Known Bug

Currently, if you set the upload URL incorrectly, let's say to something like `https://example.com/uploads/`, then `cr upload` will appear to work, but the release will not be complete. When everything is working there should be 3 assets in each release, but instead there will only be the 2 source code assets. The third asset, which is what helm actually uses, is missing. This issue will become apparent when you run `cr index` and it always claims that nothing has changed, because it can't find the asset it expects for the release.

It appears like the [go-github Do call](https://github.com/google/go-github/blob/master/github/github.go#L520) does not catch the fact that the upload URL is incorrect and pass back the expected error. If the asset upload fails, it would be better if the release was rolled back (deleted) and an appropriate log message is be displayed to the user.

The `cr index` command should also generate a warning when a release has no assets attached to it, to help people detect and troubleshoot this type of problem.
