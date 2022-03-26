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
	{ID: "go", Language: "Go", Owner: "golang", Name: "go"},
	{ID: "ruby", Language: "Ruby", Owner: "ruby", Name: "ruby"},
	{ID: "typescript", Language: "TypeScript", Owner: "microsoft", Name: "TypeScript"},
}

const maxItems = 30

func main() {
	githubToken := os.Getenv("GRAPHQL_API_GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatalln("GRAPHQL_API_GITHUB_TOKEN env var must be set")
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

	for _, repo := range repos {
		logEntry := log.WithField("repoID", repo.ID)

		logEntry.Info("Creating feed from repo...")
		feed, err := createFeedFromGithubGraphQLAPI(ctx, clientGraphQL, repo)
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

		if err := feed.WriteAtom(bufio.NewWriter(f)); err != nil {
			logEntry.WithError(err).Fatalln("Failed to write rss file")
		}
	}
}

func createFeedFromGithubGraphQLAPI(ctx context.Context, client *githubv4.Client, repo *GithubRepo) (*feeds.Feed, error) {
	var q struct {
		RateLimit struct {
			Cost      int
			Remaining int
		}
		Repository struct {
			URL string
			Ref struct {
				Edges []struct {
					Node struct {
						Name   string
						Target struct {
							Tag struct {
								Tagger struct {
									Date time.Time
								} `graphql:"tagger"`
							} `graphql:"... on Tag"`
							Commit struct {
								CommittedDate time.Time
							} `graphql:"... on Commit"`
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
	log.Debugf("Query cost: %d, remaining queries: %d\n", q.RateLimit.Cost, q.RateLimit.Remaining)

	feed := &feeds.Feed{
		Title:       fmt.Sprintf("%s releases", repo.Language),
		Link:        &feeds.Link{Href: fmt.Sprintf("%s/tags", q.Repository.URL)},
		Description: fmt.Sprintf("%s releases", repo.Language),
		Updated:     time.Now().UTC(),
	}

	edges := q.Repository.Ref.Edges
	log.Debugf("Retrieved %d items", len(edges))

	for _, edge := range edges {
		tagName := edge.Node.Name

		item := &feeds.Item{
			Title:   tagName,
			Link:    &feeds.Link{Href: fmt.Sprintf("%s/releases/tag/%s", q.Repository.URL, tagName)},
		}

		tagDate := edge.Node.Target.Tag.Tagger.Date.UTC()
		if tagDate.IsZero() {
			item.Created = edge.Node.Target.Commit.CommittedDate.UTC()
		} else {
			item.Created = tagDate
		}

		feed.Items = append(feed.Items, item)
	}

	return feed, nil
}
