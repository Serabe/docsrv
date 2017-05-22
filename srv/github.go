package srv

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var ErrNotFound = errors.New("unable to find a release")

// Release represents a project release with a tag name and an URL to the
// documentation asset.
type Release struct {
	// Tag of the release.
	Tag string
	// Docs is the URL to the .tar.gz file with the documentation.
	Docs string
}

// GitHub is a service to retrieve information from GitHub.
type GitHub interface {
	// Releases returns the latest 100 releases of a project that contain a
	// "docs.tar.gz" asset.
	// If `all` is true, all releases will be fetch.
	Releases(project string, all bool) ([]*Release, error)
	// Release returns the requested release of a project that contains a
	// "docs.tar.gz" asset.
	Release(project, tag string) (*Release, error)
}

type gitHub struct {
	apiKey string
	org    string
	client *github.Client
}

// NewGitHub creates a new GitHub service.
func NewGitHub(apiKey, org string) GitHub {
	var client *github.Client

	if apiKey != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
		client = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		client = github.NewClient(nil)
	}

	return &gitHub{apiKey, org, client}
}

func (g *gitHub) Releases(project string, all bool) ([]*Release, error) {
	var result []*Release
	page := 1
	for {
		releases, resp, err := g.client.Repositories.ListReleases(
			context.Background(),
			g.org,
			project,
			&github.ListOptions{Page: page, PerPage: 100},
		)

		if err != nil {
			return nil, err
		}

		for _, r := range releases {
			release := toRelease(r)
			if release == nil {
				continue
			}
			result = append(result, release)
		}

		if !all || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	sort.Sort(byTag(result))
	return result, nil
}

func (g *gitHub) Release(project, tag string) (*Release, error) {
	release, resp, err := g.client.Repositories.GetReleaseByTag(
		context.Background(),
		g.org,
		project,
		tag,
	)

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	if r := toRelease(release); r != nil {
		return r, nil
	}

	return nil, ErrNotFound
}

func toRelease(r *github.RepositoryRelease) *Release {
	if maybeBool(r.Draft) || maybeBool(r.Prerelease) {
		return nil
	}

	var docsURL string
	for _, a := range r.Assets {
		if maybeStr(a.Name) == "docs.tar.gz" {
			docsURL = maybeStr(a.BrowserDownloadURL)
			break
		}
	}

	if docsURL == "" {
		return nil
	}

	return &Release{
		Tag:  maybeStr(r.TagName),
		Docs: docsURL,
	}
}

type byTag []*Release

func (b byTag) Len() int      { return len(b) }
func (b byTag) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byTag) Less(i, j int) bool {
	vi := newVersion(b[i].Tag)
	vj := newVersion(b[j].Tag)
	return vi.LessThan(vj)
}

func maybeBool(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

func maybeStr(str *string) string {
	if str != nil {
		return *str
	}
	return ""
}

func newVersion(v string) *semver.Version {
	return semver.MustParse(v)
}
