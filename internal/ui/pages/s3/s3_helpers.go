package s3

import (
	"fmt"
	"os"
	"strings"

	domains3 "aws-terminal/internal/domain/s3"
)

const (
	largeDeleteConfirmationThreshold = 10
	deleteConfirmationText           = "DELETE"
)

func syncPercent(progress domains3.SyncProgress) float64 {
	if progress.Stage == "uploading" && progress.TotalUploadBytes > 0 {
		return clampPercent(float64(progress.UploadedBytes) / float64(progress.TotalUploadBytes))
	}
	if progress.Total <= 0 {
		return 0
	}

	return clampPercent(float64(progress.Current) / float64(progress.Total))
}

func clampPercent(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > 1 {
		return 1
	}

	return percent
}

func requiresDeleteConfirmation(plan *domains3.SyncPlan) bool {
	return plan != nil && plan.DeleteEnabled && plan.DeleteCount() >= largeDeleteConfirmationThreshold
}

func deleteConfirmationMatches(value string) bool {
	return strings.TrimSpace(value) == deleteConfirmationText
}

func preferredFilePickerDirectory(preferred string) string {
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		if info, err := os.Stat(preferred); err == nil && info.IsDir() {
			return preferred
		}
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}

	return "."
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size) / 1024
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}

	return fmt.Sprintf("%.1f %s", value, units[unitIndex])
}
