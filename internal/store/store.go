package store

type URLStore interface {
	AddURL(shortCode string, url string) error
	GetURL(shortCode string) (string, bool)
}
