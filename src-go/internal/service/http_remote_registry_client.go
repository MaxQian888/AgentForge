package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
)

type HTTPRemoteRegistryClient struct {
	client *http.Client
}

func NewHTTPRemoteRegistryClient(client *http.Client) *HTTPRemoteRegistryClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPRemoteRegistryClient{client: client}
}

func (c *HTTPRemoteRegistryClient) FetchCatalog(ctx context.Context, registryURL string) ([]RemotePluginEntry, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(registryURL, "/")+"/v1/plugins", nil)
	if err != nil {
		return nil, err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote registry returned %s", response.Status)
	}

	var entries []RemotePluginEntry
	if err := json.NewDecoder(response.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode remote registry catalog: %w", err)
	}
	return entries, nil
}

func (c *HTTPRemoteRegistryClient) Download(ctx context.Context, pluginID, version, registryURL string) (io.ReadCloser, error) {
	requestURL := fmt.Sprintf(
		"%s/v1/plugins/%s/versions/%s/manifest",
		strings.TrimRight(registryURL, "/"),
		neturl.PathEscape(pluginID),
		neturl.PathEscape(version),
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, fmt.Errorf("remote registry returned %s", response.Status)
	}

	return response.Body, nil
}
