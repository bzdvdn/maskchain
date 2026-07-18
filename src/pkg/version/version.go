package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Info() string {
	return "MaskChain " + Version + " (commit: " + Commit + ", built: " + Date + ")"
}
