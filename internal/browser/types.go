package browser

type Type string

const (
	Chrome   Type = "chrome"
	Chromium Type = "chromium"
	Edge     Type = "edge"
	Firefox  Type = "firefox"
	Safari   Type = "safari"
	Brave   Type = "brave"
	Vivaldi Type = "vivaldi"
	Auto    Type = "auto"
)

// Browser represents a detected browser with its database path and profile name
type Browser struct {
	Type    Type
	Name    string
	Path    string
	Profile string
}
