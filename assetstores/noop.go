package assetstores

type NOOPProvider struct{}

func NewNOOPProvider() (*NOOPProvider, error) {
	return &NOOPProvider{}, nil
}

func (n *NOOPProvider) SignURL(url string) (string, error) {
	return url, nil
}
