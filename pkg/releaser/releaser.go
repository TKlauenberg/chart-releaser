// Copyright The Helm Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package releaser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"text/template"

	"helm.sh/helm/v3/pkg/chart"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/tklauenberg/chart-releaser/pkg/config"

	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/tklauenberg/chart-releaser/pkg/github"
)

// GitHub contains the functions necessary for interacting with GitHub release
// objects
type GitHub interface {
	CreateRelease(ctx context.Context, input *github.Release) error
	GetRelease(ctx context.Context, tag string) (*github.Release, error)
	GetReleases(ctx context.Context) ([]*github.Release, error)
	CreatePullRequest(owner string, repo string, message string, head string, base string) (string, error)
}

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type Git interface {
	AddWorktree(workingDir string, committish string) (string, error)
	RemoveWorktree(workingDir string, path string) error
	Add(workingDir string, args ...string) error
	Commit(workingDir string, message string) error
	Push(workingDir string, args ...string) error
	GetPushURL(remote string, token string) (string, error)
}

type DefaultHTTPClient struct{}

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

const chartAssetFileExtension = ".tgz"

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano())) // nolint: gosec
}

func (c *DefaultHTTPClient) Get(url string) (resp *http.Response, err error) {
	return http.Get(url) // nolint: gosec
}

type Releaser struct {
	config *config.Options
	github GitHub
	git    Git
}

func NewReleaser(config *config.Options, github GitHub, git Git) *Releaser {
	return &Releaser{
		config: config,
		github: github,
		git:    git,
	}
}

// UpdateIndexFile updates the index.yaml file for a given Git repo
func (r *Releaser) UpdateIndexFile() (bool, error) {
	var worktree = ""
	indexYamlPath := filepath.Join(worktree, "index.yaml")

	var indexFile = repo.NewIndexFile()

	releases, err := r.github.GetReleases(context.TODO())

	if err != nil {
		return false, err
	}

	for _, release := range releases {
		fmt.Printf("Found Release: %s", release.Name)
	}

	var update bool
	for _, release := range releases {
		for _, asset := range release.Assets {
			downloadURL, _ := url.Parse(asset.URL)
			name := filepath.Base(downloadURL.Path)
			// Ignore any other files added in the release by the users.
			if filepath.Ext(name) != chartAssetFileExtension {
				continue
			}
			baseName := strings.TrimSuffix(name, filepath.Ext(name))
			tagParts := r.splitPackageNameAndVersion(baseName)
			packageName, packageVersion := tagParts[0], tagParts[1]
			fmt.Printf("Found %s-%s.tgz\n", packageName, packageVersion)
			if _, err := indexFile.Get(packageName, packageVersion); err != nil {
				if err := r.addToIndexFile(indexFile, downloadURL.String()); err != nil {
					return false, err
				}
				update = true
				break
			}
		}
	}

	if !update {
		fmt.Printf("Index %s did not change\n", r.config.IndexPath)
		return false, nil
	}

	// Create the directory if it doesn't exist
	err = os.MkdirAll(filepath.Dir(r.config.IndexPath), os.ModePerm)
	if err != nil {
		return false, fmt.Errorf("error creating directory: %w", err)
	}

	fmt.Printf("Updating index %s\n", r.config.IndexPath)
	indexFile.SortEntries()

	indexFile.Generated = time.Now()

	if err := indexFile.WriteFile(r.config.IndexPath, 0644); err != nil {
		return false, err
	}

	if !r.config.Push && !r.config.PR {
		return true, nil
	}

	if err := copyFile(r.config.IndexPath, indexYamlPath); err != nil {
		return false, err
	}
	if err := r.git.Add(worktree, indexYamlPath); err != nil {
		return false, err
	}
	if err := r.git.Commit(worktree, fmt.Sprintf("Update %s", r.config.PagesIndexPath)); err != nil {
		return false, err
	}

	pushURL, err := r.git.GetPushURL(r.config.Remote, r.config.Token)
	if err != nil {
		return false, err
	}

	if r.config.Push {
		fmt.Printf("Pushing to branch %q\n", r.config.PagesBranch)
		if err := r.git.Push(worktree, pushURL, "HEAD:refs/heads/"+r.config.PagesBranch); err != nil {
			return false, err
		}
	} else if r.config.PR {
		branch := fmt.Sprintf("chart-releaser-%s", randomString(16))

		fmt.Printf("Pushing to branch %q\n", branch)
		if err := r.git.Push(worktree, pushURL, "HEAD:refs/heads/"+branch); err != nil {
			return false, err
		}
		fmt.Printf("Creating pull request against branch %q\n", r.config.PagesBranch)
		prURL, err := r.github.CreatePullRequest(r.config.Owner, r.config.GitRepo, "Update index.yaml", branch, r.config.PagesBranch)
		if err != nil {
			return false, err
		}
		fmt.Println("Pull request created:", prURL)
	}

	return true, nil
}

func (r *Releaser) computeReleaseName(chart *chart.Chart) (string, error) {
	tmpl, err := template.New("gotpl").Parse(r.config.ReleaseNameTemplate)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, chart.Metadata); err != nil {
		return "", err
	}

	releaseName := buffer.String()
	return releaseName, nil
}

func (r *Releaser) getReleaseNotes(chart *chart.Chart) string {
	if r.config.ReleaseNotesFile != "" {
		for _, f := range chart.Files {
			if f.Name == r.config.ReleaseNotesFile {
				return string(f.Data)
			}
		}
		fmt.Printf("The release note file %q, is not present in the chart package\n", r.config.ReleaseNotesFile)
	}
	return chart.Metadata.Description
}

func (r *Releaser) splitPackageNameAndVersion(pkg string) []string {
	delimIndex := strings.LastIndex(pkg, "-")
	return []string{pkg[0:delimIndex], pkg[delimIndex+1:]}
}

func (r *Releaser) DownloadFile(urlStr string) (string, error) {
	filePath := filepath.Join(r.config.PackagePath, filepath.Base(urlStr))

	// Create the directory if it doesn't exist
	err := os.MkdirAll(r.config.PackagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("error creating directory: %w", err)
	}

	// Check if the file already exists
	if _, err := os.Stat(filePath); err == nil {
		fmt.Println("File already exists:", filePath)
		return filePath, nil
	}

	// Create the output file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	// Validate and parse the URL
	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Send an HTTP GET request
	response, err := http.Get(parsedURL.String())
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer response.Body.Close()

	// Check if the response was successful
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error response: %s", response.Status)
	}

	// Copy the response body to the file
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return "", fmt.Errorf("error saving file: %w", err)
	}

	return filePath, nil
}

func (r *Releaser) addToIndexFile(indexFile *repo.IndexFile, url string) error {
	arch, err := r.DownloadFile(url)

	if err != nil {
		return errors.Wrapf(err, "err in download")
	}

	// extract chart metadata
	fmt.Printf("Extracting chart metadata from %s\n", arch)
	c, err := loader.LoadFile(arch)
	if err != nil {
		return errors.Wrapf(err, "%s is not a helm chart package", arch)
	}
	// calculate hash
	fmt.Printf("Calculating Hash for %s\n", arch)
	hash, err := provenance.DigestFile(arch)
	if err != nil {
		return err
	}

	// remove url name from url as helm's index library
	// adds it in during .Add
	// there should be a better way to handle this :(
	s := strings.Split(url, "/")
	s = s[:len(s)-1]

	// Add to index
	return indexFile.MustAdd(c.Metadata, filepath.Base(arch), strings.Join(s, "/"), hash)
}

// CreateReleases finds and uploads Helm chart packages to GitHub
func (r *Releaser) CreateReleases() error {
	packages, err := r.getListOfPackages(r.config.PackagePath)
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		return errors.Errorf("no charts found at %s", r.config.PackagePath)
	}

	for _, p := range packages {
		ch, err := loader.LoadFile(p)
		if err != nil {
			return err
		}
		releaseName, err := r.computeReleaseName(ch)
		if err != nil {
			return err
		}

		release := &github.Release{
			Name:        releaseName,
			Description: r.getReleaseNotes(ch),
			Assets: []*github.Asset{
				{Path: p},
			},
			Commit:               r.config.Commit,
			GenerateReleaseNotes: r.config.GenerateReleaseNotes,
			MakeLatest:           strconv.FormatBool(r.config.MakeReleaseLatest),
		}
		provFile := fmt.Sprintf("%s.prov", p)
		if _, err := os.Stat(provFile); err == nil {
			asset := &github.Asset{Path: provFile}
			release.Assets = append(release.Assets, asset)
		}
		if r.config.SkipExisting {
			existingRelease, _ := r.github.GetRelease(context.TODO(), releaseName)
			if existingRelease != nil {
				continue
			}
		}
		if err := r.github.CreateRelease(context.TODO(), release); err != nil {
			return errors.Wrapf(err, "error creating GitHub release %s", releaseName)
		}
	}

	return nil
}

func (r *Releaser) getListOfPackages(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, "*.tgz"))
}

func copyFile(srcFile string, dstFile string) error {
	source, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))] // nolint: gosec
	}
	return string(b)
}
