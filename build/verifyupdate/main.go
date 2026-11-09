// Command verifyupdate validates native MyGo archives and optional signed deltas.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type signedAsset struct {
	URL, Signature string
	Size           int64
}
type delta struct {
	signedAsset
	From string
}
type manifest struct {
	signedAsset
	Version string
	Deltas  []delta
}
type config struct {
	Updates struct{ PublicKey, GitHub string }
}

var stableVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func main() {
	dir := flag.String("dir", "", "release asset directory")
	version := flag.String("version", "", "release version")
	configPath := flag.String("config", "mygo.json", "MyGo configuration")
	flag.Parse()
	if err := verify(*dir, *version, *configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func verify(dir, version, configPath string) error {
	if !stableVersion.MatchString(version) {
		return errors.New("invalid release version")
	}
	var cfg config
	if err := readJSON(configPath, &cfg); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(cfg.Updates.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("invalid update public key")
	}
	baseURL := "https://github.com/" + cfg.Updates.GitHub + "/releases/download/v" + version + "/"
	for _, target := range []string{"windows-amd64", "windows-arm64", "darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"} {
		var m manifest
		if err := readJSON(filepath.Join(dir, "update-"+target+".json"), &m); err != nil {
			return err
		}
		if m.Version != version {
			return fmt.Errorf("%s: incorrect version", target)
		}
		archive := "liuguangtv-" + version + "-" + target + ".tar.gz"
		if err := verifySigned(dir, archive, baseURL, key, m.signedAsset); err != nil {
			return fmt.Errorf("%s: %w", target, err)
		}
		if err := verifyContents(dir, version, target, archive); err != nil {
			return fmt.Errorf("%s: %w", target, err)
		}
		seen := map[string]bool{}
		for _, d := range m.Deltas {
			if !stableVersion.MatchString(d.From) || d.From == version || seen[d.From] {
				return fmt.Errorf("%s: invalid or duplicate delta base", target)
			}
			seen[d.From] = true
			name := "liuguangtv-" + d.From + "-to-" + version + "-" + target + ".delta"
			if err := verifySigned(dir, name, baseURL, key, d.signedAsset); err != nil {
				return fmt.Errorf("%s: delta from %s: %w", target, d.From, err)
			}
		}
	}
	return nil
}

func verifySigned(dir, name, baseURL string, key ed25519.PublicKey, asset signedAsset) error {
	if asset.URL != baseURL+name || asset.Size <= 0 {
		return errors.New("incorrect update URL or size")
	}
	file, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	signature, err := base64.StdEncoding.DecodeString(asset.Signature)
	if err != nil || n != asset.Size || !ed25519.Verify(key, hash.Sum(nil), signature) {
		return errors.New("invalid update size or signature")
	}
	return nil
}

func readJSON(name string, v any) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(v)
}

func verifyContents(dir, version, target, archive string) error {
	system, arch, _ := strings.Cut(target, "-")
	exe, installer := "", ""
	switch system {
	case "windows":
		exe, installer = "LiuguangTV.exe", "liuguangtv-setup-"+version+"-"+arch+".exe"
	case "darwin":
		exe, installer = "LiuguangTV.app/Contents/MacOS/LiuguangTV", "liuguangtv-"+version+"-"+arch+".dmg"
	case "linux":
		exe, installer = "liuguangtv", "liuguangtv_"+version+"_"+arch+".deb"
	default:
		return errors.New("unsupported target")
	}
	if info, err := os.Stat(filepath.Join(dir, installer)); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("missing installer %s", installer)
	}
	archiveHash, err := tarFileHash(filepath.Join(dir, archive), exe)
	if err != nil {
		return err
	}
	if system == "windows" {
		zipName := "liuguangtv-portable-" + version + "-" + target + ".zip"
		zipHash, err := zipFileHash(filepath.Join(dir, zipName), exe)
		if err != nil {
			return err
		}
		if archiveHash != zipHash {
			return errors.New("portable and update executables differ")
		}
	}
	return nil
}

func safePath(name string) bool {
	clean := path.Clean(name)
	return !path.IsAbs(name) && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.ContainsAny(name, "\\:")
}

func privateEntry(name string) bool {
	clean := strings.ToLower(path.Clean(name))
	return clean == "data" || strings.HasPrefix(clean, "data/") || path.Base(clean) == "portable.flag" || path.Base(clean) == "liuguangtv-portable.mode"
}

func tarFileHash(name, wanted string) ([32]byte, error) {
	var result [32]byte
	f, err := os.Open(name)
	if err != nil {
		return result, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return result, err
	}
	defer gz.Close()
	tr, found := tar.NewReader(gz), false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
		if !safePath(h.Name) || privateEntry(h.Name) {
			return result, errors.New("unsafe archive path or portable data/marker in update")
		}
		if path.Clean(h.Name) != wanted {
			continue
		}
		if found || h.Typeflag != tar.TypeReg {
			return result, errors.New("invalid or duplicate executable")
		}
		found = true
		hash := sha256.New()
		if _, err := io.Copy(hash, tr); err != nil {
			return result, err
		}
		copy(result[:], hash.Sum(nil))
	}
	if !found {
		return result, fmt.Errorf("missing executable %s", wanted)
	}
	return result, nil
}

func zipFileHash(name, wanted string) ([32]byte, error) {
	var result [32]byte
	z, err := zip.OpenReader(name)
	if err != nil {
		return result, err
	}
	defer z.Close()
	found, marker := false, false
	for _, f := range z.File {
		if !safePath(f.Name) || (privateEntry(f.Name) && f.Name != "portable.flag") {
			return result, errors.New("unsafe portable archive entry")
		}
		if f.Name == "portable.flag" {
			if marker || !f.Mode().IsRegular() {
				return result, errors.New("invalid portable marker")
			}
			marker = true
		}
		if f.Name != wanted {
			continue
		}
		if found || !f.Mode().IsRegular() {
			return result, errors.New("invalid or duplicate executable")
		}
		found = true
		r, err := f.Open()
		if err != nil {
			return result, err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, r)
		closeErr := r.Close()
		if err != nil || closeErr != nil {
			return result, errors.Join(err, closeErr)
		}
		copy(result[:], hash.Sum(nil))
	}
	if !found || !marker {
		return result, errors.New("missing portable executable or marker")
	}
	return result, nil
}
