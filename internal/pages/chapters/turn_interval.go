package chapters

import "fmt"

func turnInterval(start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%d", start)
	}
	if end == 255 {
		return fmt.Sprintf("%d → ∞", start)
	}
	return fmt.Sprintf("%d → %d", start, end)
}
