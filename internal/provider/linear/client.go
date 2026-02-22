package linear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const graphqlEndpoint = "https://api.linear.app/graphql"

// Issue holds the fields we care about from a Linear issue.
type Issue struct {
	ID          string
	Title       string
	Description string
	Identifier  string // e.g. "ENG-123"
	URL         string
	TeamKey     string // e.g. "ENG"
}

// Client is a minimal Linear GraphQL API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Linear API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlResponse struct {
	Data   json.RawMessage   `json:"data"`
	Errors []json.RawMessage `json:"errors,omitempty"`
}

func (c *Client) do(req gqlRequest) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal gql request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, graphqlEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linear api status %d: %s", resp.StatusCode, respBody)
	}

	var gqlResp gqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("unmarshal gql response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("linear graphql errors: %s", gqlResp.Errors[0])
	}

	return gqlResp.Data, nil
}

// FetchIssue retrieves issue details by ID.
func (c *Client) FetchIssue(id string) (*Issue, error) {
	const query = `
	query($id: String!) {
		issue(id: $id) {
			id
			title
			description
			identifier
			url
			team {
				key
			}
		}
	}`

	data, err := c.do(gqlRequest{
		Query:     query,
		Variables: map[string]any{"id": id},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Issue struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Identifier  string `json:"identifier"`
			URL         string `json:"url"`
			Team        struct {
				Key string `json:"key"`
			} `json:"team"`
		} `json:"issue"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal issue: %w", err)
	}

	return &Issue{
		ID:          result.Issue.ID,
		Title:       result.Issue.Title,
		Description: result.Issue.Description,
		Identifier:  result.Issue.Identifier,
		URL:         result.Issue.URL,
		TeamKey:     result.Issue.Team.Key,
	}, nil
}

// PostComment adds a comment to a Linear issue.
func (c *Client) PostComment(issueID, body string) error {
	const mutation = `
	mutation($issueId: String!, $body: String!) {
		commentCreate(input: { issueId: $issueId, body: $body }) {
			success
		}
	}`

	_, err := c.do(gqlRequest{
		Query: mutation,
		Variables: map[string]any{
			"issueId": issueID,
			"body":    body,
		},
	})
	return err
}
