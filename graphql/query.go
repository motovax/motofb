package graphql

// QueryRequest represents a single GraphQL batch query entry.
type QueryRequest struct {
	Priority     int
	Query        string
	QueryID      string
	Doc          string
	DocID        string
	QueryParams  map[string]any
}

// FromDocID creates a doc_id query (most common in fbchat-muqit).
func FromDocID(docID string, params map[string]any) QueryRequest {
	if params == nil {
		params = map[string]any{}
	}
	return QueryRequest{DocID: docID, QueryParams: params}
}