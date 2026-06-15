package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	domainupdate "aws-terminal/internal/domain/update"
)

const binaryName = "aws-terminal"

type Installer struct {
	httpClient *http.Client
}

func NewInstaller() *Installer {
	return &Installer{httpClient: &http.Client{Timeout: 60 * time.Second}}
}

func (i *Installer) InstallInstructions(ctx context.Context) (domainupdate.InstallResult, error) {
	info, err := detectInstall()
	if err != nil {
		return domainupdate.InstallResult{}, err
	}
	return domainupdate.InstallResult{
		InstallMethod:  info.method,
		SelfUpdatable:  info.selfUpdatable,
		ExecutablePath: info.path,
		Instructions:   instructions(info.method),
	}, nil
}

func (i *Installer) Install(ctx context.Context, release domainupdate.Release, currentVersion string) (domainupdate.InstallResult, error) {
	info, err := detectInstall()
	if err != nil {
		return domainupdate.InstallResult{}, err
	}

	result := domainupdate.InstallResult{
		CurrentVersion: currentVersion,
		LatestVersion:  release.Version,
		InstallMethod:  info.method,
		SelfUpdatable:  info.selfUpdatable,
		ExecutablePath: info.path,
		Instructions:   instructions(info.method),
	}
	if !info.selfUpdatable {
		return result, nil
	}

	assetName := releaseAssetName(release.Version)
	archiveURL := assetURL(release.Assets, assetName)
	checksumsURL := assetURL(release.Assets, "checksums.txt")
	if archiveURL == "" {
		return result, fmt.Errorf("release asset %q not found", assetName)
	}
	if checksumsURL == "" {
		return result, fmt.Errorf("release checksums.txt not found")
	}

	tmpDir, err := os.MkdirTemp("", "aws-terminal-update-*")
	if err != nil {
		return result, fmt.Errorf("create update temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	if err := i.download(ctx, archiveURL, archivePath); err != nil {
		return result, err
	}
	if err := i.download(ctx, checksumsURL, checksumsPath); err != nil {
		return result, err
	}
	if err := verifyChecksum(archivePath, checksumsPath, assetName); err != nil {
		return result, err
	}

	extractedPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return result, err
	}
	if err := replaceExecutable(extractedPath, info.path); err != nil {
		return result, err
	}

	result.Updated = true
	result.RestartRequired = true
	result.Instructions = "Restart aws-terminal to use the new version."
	return result, nil
}

func (i *Installer) download(ctx context.Context, url, path string) error {
	client := i.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(resp.Body, 200<<20)); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

type installInfo struct {
	method        string
	selfUpdatable bool
	path          string
}

func detectInstall() (installInfo, error) {
	path, err := os.Executable()
	if err != nil {
		return installInfo{}, fmt.Errorf("resolve executable path: %w", err)
	}
	path, _ = filepath.EvalSymlinks(path)
	lower := strings.ToLower(filepath.ToSlash(path))

	switch {
	case strings.Contains(lower, "/cellar/aws-terminal/") || strings.Contains(lower, "/homebrew/cellar/aws-terminal/"):
		return installInfo{method: domainupdate.InstallMethodHomebrew, path: path}, nil
	case strings.Contains(lower, "/scoop/"):
		return installInfo{method: domainupdate.InstallMethodScoop, path: path}, nil
	case runtime.GOOS == "windows":
		return installInfo{method: domainupdate.InstallMethodWindows, path: path}, nil
	default:
		return installInfo{method: domainupdate.InstallMethodDirect, selfUpdatable: true, path: path}, nil
	}
}

func instructions(method string) string {
	switch method {
	case domainupdate.InstallMethodHomebrew:
		return "Run: brew upgrade aws-terminal"
	case domainupdate.InstallMethodScoop:
		return "Run: scoop update aws-terminal"
	case domainupdate.InstallMethodWindows:
		return "Windows self-update is not supported yet. Use Scoop or download the latest Windows zip from the GitHub release."
	default:
		return "Install the latest release from GitHub or rerun the install script."
	}
}

func releaseAssetName(version string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s_%s_windows_%s.zip", binaryName, version, runtime.GOARCH)
	}
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", binaryName, version, runtime.GOOS, runtime.GOARCH)
}

func assetURL(assets []domainupdate.Asset, name string) string {
	for _, asset := range assets {
		if asset.Name == name {
			return asset.URL
		}
	}
	return ""
}

func verifyChecksum(archivePath, checksumsPath, assetName string) error {
	payload, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for _, line := range strings.Split(string(payload), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			expected = fields[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum for %s not found", assetName)
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	return nil
}

func extractBinary(archivePath, tmpDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZipBinary(archivePath, tmpDir)
	}
	return extractTarGzBinary(archivePath, tmpDir)
}

func extractTarGzBinary(archivePath, tmpDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("read gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		outPath := filepath.Join(tmpDir, "extract", binaryName)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
			return "", err
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return outPath, nil
	}
	return "", fmt.Errorf("archive did not contain %s", binaryName)
}

func extractZipBinary(archivePath, tmpDir string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open zip archive: %w", err)
	}
	defer zr.Close()
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != binaryName+".exe" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		outPath := filepath.Join(tmpDir, "extract", binaryName+".exe")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
			rc.Close()
			return "", err
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rcErr := rc.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if rcErr != nil {
			return "", rcErr
		}
		return outPath, nil
	}
	return "", fmt.Errorf("archive did not contain %s.exe", binaryName)
}

func replaceExecutable(sourcePath, targetPath string) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}
	if err := os.Chmod(sourcePath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod replacement executable: %w", err)
	}
	backupPath := targetPath + ".old"
	_ = os.Remove(backupPath)
	if err := os.Rename(targetPath, backupPath); err != nil {
		return fmt.Errorf("move current executable aside: %w", err)
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("replace executable: %w", err)
	}
	_ = os.Remove(backupPath)
	return nil
}
