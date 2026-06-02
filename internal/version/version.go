package version

var (
	Version   = "dev"
	Commit    = "unknown"
	Branch    = "local"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	BuildTime string `json:"buildTime"`
}

func Current() Info {
	return Info{
		Version:   valueOrDefault(Version, "dev"),
		Commit:    valueOrDefault(Commit, "unknown"),
		Branch:    valueOrDefault(Branch, "local"),
		BuildTime: valueOrDefault(BuildTime, "unknown"),
	}
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
