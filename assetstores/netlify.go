package assetstores

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type netlifyProvider struct {
	client *http.Client
	token  string
}

func newNetlifyProvider(token string) (*netlifyProvider, error) {
	if token == "" {
		return nil, errors.New("No access token configured for Netlify")
	}

	return &netlifyProvider{
		client: &http.Client{},
		token:  token,
	}, nil
}

type netlifySignature struct {
	URL string `json:"url"`
}

func (n *netlifyProvider) SignURL(downloadURL string) (string, error) {
	url, err := url.Parse(downloadURL)
	if err != nil {
		return "", err
	}
	if url.Host != "api.netlify.com" {
		return "", errors.New("Download URL didn't match Netlify API")
	}
	url.Scheme = "https"

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return "", errors.Wrap(err, "Error creating signing request")
	}
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
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		if _, err = buf.ReadFrom(resp.Body); err != nil {
			return "", fmt.Errorf("Error generating signature")
		}
		return "", fmt.Errorf("Error generating signature: %v", buf.String())
	}
	signature := &netlifySignature{}
	if err := json.NewDecoder(resp.Body).Decode(signature); err != nil {
		return "", err
	}

	return signature.URL, nil
}
