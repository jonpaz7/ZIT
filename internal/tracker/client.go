package tracker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TrackerClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *TrackerClient {
	return NewClientWithHTTPClient(baseURL, &http.Client{
		Timeout: 10 * time.Second,
	})
}

func NewClientWithHTTPClient(baseURL string, httpClient *http.Client) *TrackerClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &TrackerClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *TrackerClient) Announce(peer string, hashes []string) error {
	if c == nil || c.httpClient == nil {
		return fmt.Errorf("tracker client not configured")
	}
	reqBody, err := json.Marshal(AnnounceRequest{
		Hashes: hashes,
		Peer:   peer,
	})
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Post(c.baseURL+"/announce", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("announce to tracker %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("announce failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *TrackerClient) Providers(hash string) ([]string, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("tracker client not configured")
	}
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/providers/%s", c.baseURL, hash))
	if err != nil {
		return nil, fmt.Errorf("get providers for %s: %w", hash, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("providers failed (%d): %s", resp.StatusCode, string(body))
	}
	var peers []string
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func (c *TrackerClient) GetRefs(repoId string) (RepoRefs, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/refs/%s", c.baseURL, repoId))
	if err != nil {
		return nil, fmt.Errorf("get refs for %s: %w", repoId, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get refs failed (%d): %s", resp.StatusCode, string(body))
	}
	var refs RepoRefs
	if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func (c *TrackerClient) PutRef(repoId, ref, commit, signature, publicKey string) error {
	refClean := strings.TrimPrefix(ref, "refs/")
	url := fmt.Sprintf("%s/refs/%s/%s", c.baseURL, repoId, refClean)

	payload := RefData{
		Commit:    commit,
		Signature: signature,
		PublicKey: publicKey,
	}
	reqData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(reqData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("put ref %s: %w", ref, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.New(fmt.Sprintf("put ref failed (%d): %s", resp.StatusCode, string(body)))
	}
	return nil
}
