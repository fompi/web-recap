package browser

import "errors"

var (
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrBrowserNotAvailable = errors.New("browser not available on this platform")
	ErrUnknownBrowser      = errors.New("unknown browser type")
	ErrDatabaseNotFound    = errors.New("database file not found")
)
