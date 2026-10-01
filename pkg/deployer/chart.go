// Package deployer loads module descriptions and deploys their Helm charts.
package deployer

import (
	"fmt"
	"os"
	"path/filepath"
)

// loadChartPath resolves a chart directory beneath root. The Helm engine
// loads and caches the chart internally when it renders.
func loadChartPath(root, relative string) (string, error) {
	if !filepath.IsLocal(relative) {
		return "", fmt.Errorf("chart path %q must be local and relative", relative)
	}
	rootPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve charts path: %w", err)
	}
	chartPath, err := filepath.EvalSymlinks(filepath.Join(rootPath, relative))
	if err != nil {
		return "", fmt.Errorf("resolve chart path: %w", err)
	}
	within, err := filepath.Rel(rootPath, chartPath)
	if err != nil || !filepath.IsLocal(within) && within != "." {
		return "", fmt.Errorf("chart path %q escapes charts path", relative)
	}
	info, err := os.Stat(chartPath)
	if err != nil {
		return "", fmt.Errorf("stat chart path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("chart path %q is not a directory", relative)
	}
	return chartPath, nil
}
