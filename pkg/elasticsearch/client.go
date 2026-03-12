// Package elasticsearch provides an Elasticsearch client with connection
// pooling, configuration management, and common search/index operations.
package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// Config holds Elasticsearch connection configuration.
type Config struct {
	Addresses []string      `yaml:"addresses" json:"addresses"`
	Username  string        `yaml:"username" json:"username"`
	Password  string        `yaml:"password" json:"password"`
	APIKey    string        `yaml:"api_key" json:"api_key"`
	CloudID   string        `yaml:"cloud_id" json:"cloud_id"`
	Timeout   time.Duration `yaml:"timeout" json:"timeout"`
	MaxRetries int          `yaml:"max_retries" json:"max_retries"`
}

// DefaultConfig returns default Elasticsearch configuration.
func DefaultConfig() Config {
	return Config{
		Addresses: []string{"http://localhost:9200"},
		Timeout:   30 * time.Second,
		MaxRetries: 3,
	}
}

// Client wraps elasticsearch.Client with additional functionality.
type Client struct {
	es     *elasticsearch.Client
	config Config
}

// New creates a new Elasticsearch client from config.
func New(config Config) (*Client, error) {
	opts := elasticsearch.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
		APIKey:    config.APIKey,
		CloudID:   config.CloudID,
		RetryBackoff: func(i int) time.Duration {
			return time.Duration(i) * 100 * time.Millisecond
		},
		MaxRetries: config.MaxRetries,
	}

	es, err := elasticsearch.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := esapi.InfoRequest{}
	if _, err := req.Do(ctx, es); err != nil {
		return nil, fmt.Errorf("failed to ping elasticsearch: %w", err)
	}

	return &Client{
		es:     es,
		config: config,
	}, nil
}

// ES returns the underlying elasticsearch.Client instance.
func (c *Client) ES() *elasticsearch.Client {
	return c.es
}

// Info returns cluster information.
func (c *Client) Info(ctx context.Context) (*esapi.Response, error) {
	req := esapi.InfoRequest{}
	return req.Do(ctx, c.es)
}

// ==================== Index Operations ====================

// Index creates or updates a document in an index.
func (c *Client) Index(ctx context.Context, index string, documentID string, document any) (*esapi.Response, error) {
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: documentID,
		Body:       bytes.NewReader(data),
		Refresh:    "true",
	}

	return req.Do(ctx, c.es)
}

// BulkIndex performs bulk indexing of documents.
func (c *Client) BulkIndex(ctx context.Context, index string, documents map[string]any) (*esapi.Response, error) {
	var buf bytes.Buffer
	for docID, doc := range documents {
		meta := map[string]any{
			"index": map[string]any{
				"_index": index,
				"_id":    docID,
			},
		}
		if err := json.NewEncoder(&buf).Encode(meta); err != nil {
			return nil, err
		}
		if err := json.NewEncoder(&buf).Encode(doc); err != nil {
			return nil, err
		}
	}

	req := esapi.BulkRequest{
		Body:    &buf,
		Refresh: "true",
	}

	return req.Do(ctx, c.es)
}

// Get retrieves a document by ID.
func (c *Client) Get(ctx context.Context, index string, documentID string) (*esapi.Response, error) {
	req := esapi.GetRequest{
		Index:      index,
		DocumentID: documentID,
	}
	return req.Do(ctx, c.es)
}

// Delete removes a document by ID.
func (c *Client) Delete(ctx context.Context, index string, documentID string) (*esapi.Response, error) {
	req := esapi.DeleteRequest{
		Index:      index,
		DocumentID: documentID,
		Refresh:    "true",
	}
	return req.Do(ctx, c.es)
}

// DeleteByQuery removes documents matching a query.
func (c *Client) DeleteByQuery(ctx context.Context, indices []string, query map[string]any) (*esapi.Response, error) {
	data, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	req := esapi.DeleteByQueryRequest{
		Index: indices,
		Body:  bytes.NewReader(data),
	}
	return req.Do(ctx, c.es)
}

// Exists checks if a document exists.
func (c *Client) Exists(ctx context.Context, index string, documentID string) (*esapi.Response, error) {
	req := esapi.ExistsRequest{
		Index:      index,
		DocumentID: documentID,
	}
	return req.Do(ctx, c.es)
}

// ==================== Search Operations ====================

// Search performs a search query.
func (c *Client) Search(ctx context.Context, indices []string, query map[string]any) (*esapi.Response, error) {
	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: indices,
		Body:  bytes.NewReader(data),
	}
	return req.Do(ctx, c.es)
}

// SearchSimple performs a simple match query.
func (c *Client) SearchSimple(ctx context.Context, index string, field string, value string) (*esapi.Response, error) {
	query := map[string]any{
		"query": map[string]any{
			"match": map[string]string{
				field: value,
			},
		},
	}
	return c.Search(ctx, []string{index}, query)
}

// Count returns the number of documents matching a query.
func (c *Client) Count(ctx context.Context, indices []string, query map[string]any) (*esapi.Response, error) {
	var body *bytes.Reader
	if query != nil {
		data, err := json.Marshal(query)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req := esapi.CountRequest{
		Index: indices,
		Body:  body,
	}
	return req.Do(ctx, c.es)
}

// ==================== Index Management ====================

// CreateIndex creates a new index with optional settings.
func (c *Client) CreateIndex(ctx context.Context, index string, settings map[string]any) (*esapi.Response, error) {
	var body *bytes.Reader
	if settings != nil {
		data, err := json.Marshal(settings)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  body,
	}
	return req.Do(ctx, c.es)
}

// DeleteIndex removes an index.
func (c *Client) DeleteIndex(ctx context.Context, indices ...string) (*esapi.Response, error) {
	req := esapi.IndicesDeleteRequest{
		Index: indices,
	}
	return req.Do(ctx, c.es)
}

// IndexExists checks if an index exists.
func (c *Client) IndexExists(ctx context.Context, index string) (*esapi.Response, error) {
	req := esapi.IndicesExistsRequest{
		Index: []string{index},
	}
	return req.Do(ctx, c.es)
}

// Refresh refreshes one or more indices.
func (c *Client) Refresh(ctx context.Context, indices ...string) (*esapi.Response, error) {
	req := esapi.IndicesRefreshRequest{
		Index: indices,
	}
	return req.Do(ctx, c.es)
}

// ==================== Aggregations ====================

// Aggregate performs an aggregation query.
func (c *Client) Aggregate(ctx context.Context, index string, aggs map[string]any) (*esapi.Response, error) {
	query := map[string]any{
		"size": 0,
		"aggs": aggs,
	}
	return c.Search(ctx, []string{index}, query)
}

// ==================== Response Helpers ====================

// IsSuccess checks if response status indicates success (2xx).
func IsSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// IsNotFound checks if response indicates document not found.
func IsNotFound(statusCode int) bool {
	return statusCode == 404
}

// ParseResponse parses ES response body into a map.
func ParseResponse(body []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BuildMatchAllQuery builds a simple match_all query.
func BuildMatchAllQuery() map[string]any {
	return map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
	}
}

// BuildTermQuery builds a term query for exact matches.
func BuildTermQuery(field string, value any) map[string]any {
	return map[string]any{
		"query": map[string]any{
			"term": map[string]any{
				field: value,
			},
		},
	}
}

// BuildRangeQuery builds a range query.
func BuildRangeQuery(field string, gte, lte any) map[string]any {
	rangeQuery := map[string]any{}
	if gte != nil {
		rangeQuery["gte"] = gte
	}
	if lte != nil {
		rangeQuery["lte"] = lte
	}

	return map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				field: rangeQuery,
			},
		},
	}
}

// BuildBoolQuery builds a bool query with must/should/must_not clauses.
func BuildBoolQuery(must, should, mustNot []map[string]any) map[string]any {
	boolQuery := map[string]any{}
	if len(must) > 0 {
		boolQuery["must"] = must
	}
	if len(should) > 0 {
		boolQuery["should"] = should
	}
	if len(mustNot) > 0 {
		boolQuery["must_not"] = mustNot
	}

	return map[string]any{
		"query": map[string]any{
			"bool": boolQuery,
		},
	}
}

// BuildMultiMatchQuery builds a multi_match query.
func BuildMultiMatchQuery(query string, fields []string, queryType string) map[string]any {
	mm := map[string]any{
		"query":  query,
		"fields": fields,
	}
	if queryType != "" {
		mm["type"] = queryType
	}

	return map[string]any{
		"query": map[string]any{
			"multi_match": mm,
		},
	}
}
