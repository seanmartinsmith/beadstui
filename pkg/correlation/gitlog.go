package correlation

const (
	gitLogHeaderFormat = "%H%x00%aI%x00%an%x00%ae%x00%s"

	// gitLogMaxScanTokenSize matches the loader and stream limits; it prevents
	// bufio.Scanner from failing on unusually long lines.
	gitLogMaxScanTokenSize = 10 * 1024 * 1024 // 10MB
)
