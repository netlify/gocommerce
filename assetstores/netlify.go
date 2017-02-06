package assetstores

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type NetlifyProvider struct {
	client *http.Client
	token  string
}

func NewNetlifyProvider(token string) (*NetlifyProvider, error) {
	if token == "" {
		return nil, errors.New("No access token configured for Netlify")
	}

	return &NetlifyProvider{
		client: &http.Client{},
		token:  token,
	}, nil
}

type NetlifySignature struct {
	URL string `json:"url"`
}

func (n *NetlifyProvider) SignURL(downloadURL string) (string, error) {
	url, err := url.Parse(downloadURL)
	if err != nil {
		return "", err
	}
	if url.Host != "api.netlify.com" {
		return "", errors.New("Download URL didn't match Netlify API")
	}
	url.Scheme = "https"

	req, err := http.NewRequest("GET", url.String(), nil)
	req.Header.Add("Authorization", "Bearer "+n.token)

	resp, err := n.client.Do(req)
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return "", fmt.Errorf("Error generating signature: %v", buf.String())
	}
	signature := &NetlifySignature{}
	if err := json.NewDecoder(resp.Body).Decode(signature); err != nil {
		return "", err
	}

	return signature.URL, nil
}
