package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func releaseFixture(t *testing.T, deltas bool) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config{}
	cfg.Updates.PublicKey = base64.StdEncoding.EncodeToString(pub)
	cfg.Updates.GitHub = "example/project"
	cfgFile := filepath.Join(dir, "config.json")
	writeJSON(t, cfgFile, cfg)
	for _, target := range []string{"windows-amd64", "windows-arm64", "darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"} {
		system, arch, _ := strings.Cut(target, "-")
		exe, installer := "liuguangtv", "liuguangtv_1.2.3_"+arch+".deb"
		if system == "windows" {
			exe, installer = "LiuguangTV.exe", "liuguangtv-setup-1.2.3-"+arch+".exe"
		} else if system == "darwin" {
			exe, installer = "LiuguangTV.app/Contents/MacOS/LiuguangTV", "liuguangtv-1.2.3-"+arch+".dmg"
		}
		data := []byte("test executable " + target)
		archive := "liuguangtv-1.2.3-" + target + ".tar.gz"
		writeTar(t, filepath.Join(dir, archive), exe, data)
		mustWrite(t, filepath.Join(dir, installer), []byte("installer"))
		if system == "windows" {
			writeZip(t, filepath.Join(dir, "liuguangtv-portable-1.2.3-"+target+".zip"), map[string][]byte{exe: data, "portable.flag": nil})
		}
		sign := func(name string) signedAsset {
			bytes, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(bytes)
			return signedAsset{URL: "https://github.com/example/project/releases/download/v1.2.3/" + name, Size: int64(len(bytes)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, hash[:]))}
		}
		m := manifest{Version: "1.2.3", signedAsset: sign(archive)}
		if deltas {
			name := "liuguangtv-1.2.2-to-1.2.3-" + target + ".delta"
			mustWrite(t, filepath.Join(dir, name), []byte("signed delta fixture "+target))
			m.Deltas = []delta{{From: "1.2.2", signedAsset: sign(name)}}
		}
		writeJSON(t, filepath.Join(dir, "update-"+target+".json"), m)
	}
	return dir, cfgFile
}

func TestVerifyRelease(t *testing.T) {
	for _, deltas := range []bool{false, true} {
		dir, cfg := releaseFixture(t, deltas)
		if err := verify(dir, "1.2.3", cfg); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(dir, "liuguangtv-1.2.3-windows-amd64.tar.gz"), []byte("tampered"))
		if err := verify(dir, "1.2.3", cfg); err == nil {
			t.Fatal("accepted tampered full archive")
		}
	}
}

func TestRejectInvalidDeltas(t *testing.T) {
	for _, mutation := range []string{"tampered", "missing", "url", "signature", "size", "duplicate", "from"} {
		t.Run(mutation, func(t *testing.T) {
			dir, cfg := releaseFixture(t, true)
			manifestPath := filepath.Join(dir, "update-windows-amd64.json")
			var m manifest
			if err := readJSON(manifestPath, &m); err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(dir, "liuguangtv-1.2.2-to-1.2.3-windows-amd64.delta")
			switch mutation {
			case "tampered":
				mustWrite(t, name, []byte("bad"))
			case "missing":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			case "url":
				m.Deltas[0].URL = "https://untrusted.example/file.delta"
			case "signature":
				m.Deltas[0].Signature = "invalid"
			case "size":
				m.Deltas[0].Size++
			case "duplicate":
				m.Deltas = append(m.Deltas, m.Deltas[0])
			case "from":
				m.Deltas[0].From = "../bad"
			}
			writeJSON(t, manifestPath, m)
			if err := verify(dir, "1.2.3", cfg); err == nil {
				t.Fatal("accepted invalid delta")
			}
		})
	}
}

func TestRejectPortableDataAndUnsafePathsInUpdate(t *testing.T) {
	for _, name := range []string{"data/state.json", "DATA/state.json", "portable.flag", "liuguangtv-portable.mode", "../outside", "C:/outside", "a\\b"} {
		p := filepath.Join(t.TempDir(), "bad.tar.gz")
		writeTar(t, p, name, []byte("not allowed"))
		if _, err := tarFileHash(p, name); err == nil {
			t.Fatalf("accepted %s in update", name)
		}
	}
}

func TestRejectIncorrectPortableZip(t *testing.T) {
	for _, entries := range []map[string][]byte{
		{"LiuguangTV.exe": []byte("different"), "portable.flag": nil},
		{"LiuguangTV.exe": []byte("test executable windows-amd64")},
		{"LiuguangTV.exe": []byte("test executable windows-amd64"), "portable.flag": nil, "data/state.json": []byte("private")},
	} {
		dir, cfg := releaseFixture(t, false)
		writeZip(t, filepath.Join(dir, "liuguangtv-portable-1.2.3-windows-amd64.zip"), entries)
		if err := verify(dir, "1.2.3", cfg); err == nil {
			t.Fatal("accepted incorrect portable zip")
		}
	}
}

func writeJSON(t *testing.T, p string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, p, b)
}
func mustWrite(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func writeTar(t *testing.T, p, n string, b []byte) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	w := tar.NewWriter(gz)
	if err = w.WriteHeader(&tar.Header{Name: n, Mode: 0755, Size: int64(len(b))}); err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(b); err != nil {
		t.Fatal(err)
	}
	for _, close := range []func() error{w.Close, gz.Close, f.Close} {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
}
func writeZip(t *testing.T, p string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, data := range entries {
		e, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}
