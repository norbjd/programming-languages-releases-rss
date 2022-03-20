# Programming languages releases RSS feed

Follow your favorite programming languages releases with RSS feeds!

See [the RSS feeds homepage](https://norbjd.github.io/programming-languages-releases-rss/) hosted on GitHub Pages.

## Generate RSS feeds

RSS feeds generation is based on [Github GraphQL API](https://docs.github.com/en/graphql). You should use a personal token to query the API.

Generate feeds by running:

```shell
GRAPHQL_API_GITHUB_TOKEN="<your-github-token>" go run main.go
```

`DEBUG` environment variable can be set for debugging purposes.
