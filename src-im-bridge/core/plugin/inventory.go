package plugin

// IMBridgeCommandPlugin mirrors the backend model IMBridgeCommandPlugin in
// wire format. The bridge package depends only on core, so we declare the
// struct locally and serialize through json tags that match backend JSON.
type IMBridgeCommandPlugin struct {
	ID         string   `json:"id"`
	Version    string   `json:"version"`
	Commands   []string `json:"commands"`
	Tenants    []string `json:"tenants,omitempty"`
	SourcePath string   `json:"sourcePath,omitempty"`
}
