package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/gorilla/feeds"
	"github.com/shurcooL/githubv4"
	log "github.com/sirupsen/logrus"
)

type GithubRepo struct {
	ID       string
	Language string
	Owner    string
	Name     string
}

var repos = []*GithubRepo{
	{ID: "nodejs", Language: "Node.js", Owner: "nodejs", Name: "node"},
	{ID: "python", Language: "Python", Owner: "python", Name: "cpython"},
	{ID: "rust", Language: "Rust", Owner: "rust-lang", Name: "rust"},
	{ID: "php", Language: "PHP", Owner: "php", Name: "php-src"},
}

const maxItems = 30

func main() {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatalln("GITHUB_TOKEN env var must be set")
	}

	if _, ok := os.LookupEnv("DEBUG"); ok {
		log.SetLevel(log.DebugLevel)
	}

	ctx := context.Background()

	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: githubToken,
		},
	))
	clientGraphQL := githubv4.NewClient(httpClient)
	now := time.Now()

	for _, repo := range repos {
		logEntry := log.WithField("repoID", repo.ID)

		logEntry.Info("Creating feed from repo...")
		feed, err := createFeedFromGithubGraphQLAPI(ctx, clientGraphQL, repo, now)
		if err != nil {
			logEntry.WithError(err).Fatalln("Failed to retrieve feed")
		}
		logEntry.Info("Feed from repo created!")

		filename := fmt.Sprintf("rss/%s.xml", repo.ID)
		logEntry = logEntry.WithField("filename", filename)

		f, err := os.Create(filename)
		if err != nil {
			logEntry.WithError(err).Fatalln("Failed to create file")
		}
		defer f.Close()

		logEntry.Info("Writing RSS file")

		if err := feed.WriteRss(bufio.NewWriter(f)); err != nil {
			logEntry.WithError(err).Fatalln("Failed to write rss file")
		}
	}
}

func createFeedFromGithubGraphQLAPI(ctx context.Context, client *githubv4.Client, repo *GithubRepo, feedCreationTime time.Time) (*feeds.Feed, error) {
	var q struct {
		Repository struct {
			URL string
			Ref struct {
				Edges []struct {
					Node struct {
						Target struct {
							Tag struct {
								Name   string
								Tagger struct {
									Date time.Time
								} `graphql:"tagger"`
							} `graphql:"... on Tag"`
						} `graphql:"target"`
					} `graphql:"node"`
				} `graphql:"edges"`
			} `graphql:"refs(refPrefix: $refPrefix, first: $count, orderBy: {field: TAG_COMMIT_DATE, direction: DESC})"`
		} `graphql:"repository(owner: $repositoryOwner, name: $repositoryName)"`
	}

	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(repo.Owner),
		"repositoryName":  githubv4.String(repo.Name),
		"refPrefix":       githubv4.String("refs/tags/"),
		"count":           githubv4.Int(maxItems),
	}

	err := client.Query(ctx, &q, variables)
	if err != nil {
		log.WithError(err).Errorln("Cannot query Github GraphQL API")
		return nil, err
	}

	log.Debugf("GraphQL query result: %+v\n", q)

	feed := &feeds.Feed{
		Title:       fmt.Sprintf("%s releases", repo.Language),
		Link:        &feeds.Link{Href: fmt.Sprintf("%s/tags", q.Repository.URL)},
		Description: fmt.Sprintf("%s releases", repo.Language),
		Created:     feedCreationTime,
	}

	edges := q.Repository.Ref.Edges
	log.Debugf("Retrieved %d items", len(edges))

	for _, edge := range edges {
		tagName := edge.Node.Target.Tag.Name

		item := &feeds.Item{
			Title:   tagName,
			Link:    &feeds.Link{Href: fmt.Sprintf("%s/releases/tag/%s", q.Repository.URL, tagName)},
			Created: edge.Node.Target.Tag.Tagger.Date,
		}

		feed.Items = append(feed.Items, item)
	}

	return feed, nil
}
