package assetstores

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
)

const NETLIFY_URL = "https://api.netlify.com/api/v1"

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

var (
	urlRegExp = regexp.MustCompile(`cloud.netlifyusercontent.com/assets/([^/]+)/([^/]+)/`)
)

func (n *NetlifyProvider) SignURL(url string) (string, error) {
	matches := urlRegExp.FindStringSubmatch(url)
	if len(matches) != 3 {
		return "", errors.New("URL didn't match a Netlify asset URL")
	}

	apiURL := NETLIFY_URL + "/sites/" + matches[1] + "/assets/" + matches[2] + "/public_signature"
	req, err := http.NewRequest("GET", apiURL, nil)
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
	signature := &NetlifySignature{}
	if err := json.NewDecoder(resp.Body).Decode(signature); err != nil {
		return "", err
	}

	return signature.URL, nil
}
