package assetstores

type noopProvider struct{}

func newNoopProvider() (*noopProvider, error) {
	return &noopProvider{}, nil
}

func (n *noopProvider) SignURL(url string) (string, error) {
	return url, nil
}
