package lyrics

// Request carries track metadata used to look up lyrics across providers.
type Request struct {
	Title       string
	Artist      string
	Album       string
	DurationSec int
	ISRC        string
	Provider    string // "" | "lrclib" | "netease"
}

// Result holds the resolved lyrics text, sync tier, and provider identity.
type Result struct {
	LRC          string // Standard LRC text
	SyncType     string // "line" | "none"
	Provider     string // "lrclib" | "netease"
	Instrumental bool
}
