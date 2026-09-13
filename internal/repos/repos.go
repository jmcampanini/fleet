// Package repos describes the configured inventory with its GitHub topics.
package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/jmcampanini/fleet/internal/process"
)

// Entry describes one configured repository. Topics is empty when they were
// not requested or could not be retrieved; Error explains a retrieval failure.
type Entry struct {
	Repository string   `json:"repository"`
	Path       string   `json:"path"`
	Branch     string   `json:"branch"`
	Groups     []string `json:"groups"`
	Topics     []string `json:"topics"`
	Error      string   `json:"error,omitempty"`
}

// Report lists the selected repositories in complete-identity order.
type Report struct {
	Complete      bool    `json:"complete"`
	TopicsFetched bool    `json:"topics_fetched"`
	Results       []Entry `json:"results"`
}

// Client retrieves topics through gh with at most four repositories in flight.
type Client struct {
	Run process.Run
}

// List describes every selected repository from the inventory and, when
// fetchTopics is set, asks GitHub for each repository's topics. A topic
// failure marks that entry and continues with the others.
func (c Client) List(ctx context.Context, root string, repos []inventory.Repository, fetchTopics bool) Report {
	report := Report{Complete: true, TopicsFetched: fetchTopics, Results: make([]Entry, len(repos))}
	for index, repo := range repos {
		report.Results[index] = Entry{
			Repository: repo.ID,
			Path:       filepath.Join(root, filepath.FromSlash(repo.ID)),
			Branch:     repo.Branch,
			Groups:     append([]string{}, repo.Groups...),
			Topics:     []string{},
		}
	}
	if !fetchTopics {
		return report
	}

	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(repos)) {
		workers.Go(func() {
			for index := range jobs {
				topics, err := c.topics(ctx, repos[index].ID)
				if err != nil {
					report.Results[index].Error = err.Error()
					continue
				}
				report.Results[index].Topics = topics
			}
		})
	}
	for index := range repos {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	for _, entry := range report.Results {
		report.Complete = report.Complete && entry.Error == ""
	}
	return report
}

func (c Client) topics(ctx context.Context, id string) ([]string, error) {
	body, err := c.Run(ctx, "", "gh", "repo", "view", id, "--json", "repositoryTopics")
	if err != nil {
		host, _, _ := strings.Cut(id, "/")
		return nil, fmt.Errorf("fetch topics for %q on host %q with gh: %w; pass --no-topics to list without contacting GitHub", id, host, err)
	}

	var raw struct {
		RepositoryTopics []struct{ Name string }
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("decode topics for %q: %w", id, err)
	}
	topics := []string{}
	for _, topic := range raw.RepositoryTopics {
		if topic.Name == "" {
			return nil, fmt.Errorf("topics for %q include an unnamed topic", id)
		}
		topics = append(topics, topic.Name)
	}
	slices.Sort(topics)
	return topics, nil
}
