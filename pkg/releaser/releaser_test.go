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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tklauenberg/chart-releaser/pkg/github"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/tklauenberg/chart-releaser/pkg/config"
)

type FakeGitHub struct {
	mock.Mock
	release *github.Release
}

type FakeGit struct {
	indexFile string
}

func (f *FakeGit) AddWorktree(workingDir string, committish string) (string, error) {
	dir, err := os.MkdirTemp("", "chart-releaser-")
	if err != nil {
		return "", err
	}
	if len(f.indexFile) == 0 {
		return dir, nil
	}

	return dir, copyFile(f.indexFile, filepath.Join(dir, "index.yaml"))
}

func (f *FakeGit) RemoveWorktree(workingDir string, path string) error {
	return nil
}

func (f *FakeGit) Add(workingDir string, args ...string) error {
	panic("implement me")
}

func (f *FakeGit) Commit(workingDir string, message string) error {
	panic("implement me")
}

func (f *FakeGit) Push(workingDir string, args ...string) error {
	panic("implement me")
}

func (f *FakeGit) GetPushURL(remote string, token string) (string, error) {
	panic("implement me")
}

func (f *FakeGitHub) CreateRelease(ctx context.Context, input *github.Release) error {
	f.Called(ctx, input)
	f.release = input
	return nil
}

func (f *FakeGitHub) GetRelease(ctx context.Context, tag string) (*github.Release, error) {
	release := &github.Release{
		Name:        "testdata/release-packages/test-chart-0.1.0",
		Description: "A Helm chart for Kubernetes",
		Assets: []*github.Asset{
			{
				Path: "testdata/release-packages/test-chart-0.1.0.tgz",
				URL:  "https://myrepo/charts/test-chart-0.1.0.tgz",
			},
			{
				Path: "testdata/release-packages/third-party-file-0.1.0.txt",
				URL:  "https://myrepo/charts/third-party-file-0.1.0.txt",
			},
		},
	}
	return release, nil
}

func (f *FakeGitHub) GetReleases(ctx context.Context) ([]*github.Release, error) {
	releases := []*github.Release{
		{
			Name:        "testdata/release-packages/test-chart-0.1.0",
			Description: "A Helm chart for Kubernetes",
			Assets: []*github.Asset{
				{
					Path: "testdata/release-packages/test-chart-0.1.0.tgz",
					URL:  "https://myrepo/charts/test-chart-0.1.0.tgz",
				},
				{
					Path: "testdata/release-packages/third-party-file-0.1.0.txt",
					URL:  "https://myrepo/charts/third-party-file-0.1.0.txt",
				},
			},
		}}
	return releases, nil
}

func (f *FakeGitHub) CreatePullRequest(owner string, repo string, message string, head string, base string) (string, error) {
	f.Called(owner, repo, message, head, base)
	return "https://github.com/owner/repo/pull/42", nil
}

func TestReleaser_splitPackageNameAndVersion(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected []string
	}{
		{
			"no-hyphen",
			"foo",
			nil,
		},
		{
			"one-hyphen",
			"foo-1.2.3",
			[]string{"foo", "1.2.3"},
		},
		{
			"two-hyphens",
			"foo-bar-1.2.3",
			[]string{"foo-bar", "1.2.3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Releaser{}
			if tt.expected == nil {
				assert.Panics(t, func() {
					r.splitPackageNameAndVersion(tt.pkg)
				}, "slice bounds out of range")
			} else {
				actual := r.splitPackageNameAndVersion(tt.pkg)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestReleaser_addToIndexFile(t *testing.T) {
	tests := []struct {
		name    string
		chart   string
		version string
		error   bool
	}{
		{
			"invalid-package",
			"does-not-exist",
			"0.1.0",
			true,
		},
		{
			"valid-package",
			"test-chart",
			"0.1.0",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Releaser{
				config: &config.Options{PackagePath: "testdata/release-packages"},
			}
			indexFile := repo.NewIndexFile()
			url := fmt.Sprintf("https://myrepo/charts/%s-%s.tgz", tt.chart, tt.version)
			err := r.addToIndexFile(indexFile, url)
			if tt.error {
				assert.Error(t, err)
				assert.False(t, indexFile.Has(tt.chart, tt.version))
			} else {
				assert.True(t, indexFile.Has(tt.chart, tt.version))
			}
		})
	}
}
