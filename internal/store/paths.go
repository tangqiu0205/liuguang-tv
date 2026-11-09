package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Path() (string, bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", false, fmt.Errorf("无法确定程序位置: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", false, fmt.Errorf("无法解析程序位置: %w", err)
	}
	portable, err := portableMode(exe, runtime.GOOS)
	if err != nil {
		return "", false, err
	}
	if !portable {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", false, err
		}
		return filepath.Join(dir, "LiuguangTV", "state.json"), false, nil
	}
	path, err := portablePath(exe, runtime.GOOS)
	if err != nil {
		return "", false, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", false, fmt.Errorf("无法创建便携版数据目录 %s: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return "", false, fmt.Errorf("便携版数据目录不可写 %s: %w", dir, err)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return "", false, fmt.Errorf("检查便携版数据目录失败: %w", err)
	}
	if err := os.Remove(name); err != nil {
		return "", false, fmt.Errorf("清理便携版数据目录检查文件失败: %w", err)
	}
	return path, true, nil
}

func windowsPortable(exe, goos string) (bool, error) {
	if goos != "windows" {
		return false, nil
	}
	dir := filepath.Dir(exe)
	marker := filepath.Join(dir, "portable.flag")
	info, err := os.Lstat(marker)
	if errors.Is(err, os.ErrNotExist) {
		if _, dataErr := os.Lstat(filepath.Join(dir, "data")); dataErr == nil {
			return false, fmt.Errorf("检测到便携数据，但缺少 %s；请恢复便携标记后重试", marker)
		} else if !errors.Is(dataErr, os.ErrNotExist) {
			return false, fmt.Errorf("检查便携数据失败: %w", dataErr)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取便携标记失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("便携标记不是普通文件: %s", marker)
	}
	if _, legacyErr := os.Lstat(filepath.Join(dir, "liuguangtv-portable.mode")); legacyErr == nil {
		return false, errors.New("检测到新旧便携标记，请保留与原数据目录对应的一种布局")
	} else if !errors.Is(legacyErr, os.ErrNotExist) {
		return false, fmt.Errorf("检查旧便携标记失败: %w", legacyErr)
	}
	return true, nil
}

// The marker is outside signed update archives, so updates preserve the mode.
func portableMode(exe, goos string) (bool, error) {
	if portable, err := windowsPortable(exe, goos); portable || err != nil {
		return portable, err
	}
	dir := filepath.Dir(exe)
	if goos == "darwin" && filepath.Base(dir) == "MacOS" && filepath.Base(filepath.Dir(dir)) == "Contents" {
		bundle := filepath.Dir(filepath.Dir(dir))
		if strings.HasSuffix(filepath.Base(bundle), ".app") {
			dir = filepath.Dir(bundle)
		}
	}
	marker := filepath.Join(dir, "liuguangtv-portable.mode")
	info, err := os.Lstat(marker)
	if errors.Is(err, os.ErrNotExist) {
		// Do not silently switch to installed data if a portable directory has
		// lost its marker. Without adjacent data we cannot distinguish modes.
		if filepath.Base(dir) == "app" {
			data := filepath.Join(filepath.Dir(dir), "data")
			if _, dataErr := os.Lstat(data); dataErr == nil {
				return false, fmt.Errorf("检测到 %s，但便携版标记缺失；若为便携版，请从可信便携包恢复 %s；若为安装版，请确认安装位置", data, marker)
			} else if !errors.Is(dataErr, os.ErrNotExist) {
				return false, fmt.Errorf("检查便携版数据目录失败: %w", dataErr)
			}
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取便携版标记失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("便携版标记不是普通文件: %s", marker)
	}
	return true, nil
}

func portablePath(exe, goos string) (string, error) {
	if portable, err := windowsPortable(exe, goos); err != nil {
		return "", err
	} else if portable {
		return filepath.Join(filepath.Dir(exe), "data", "state.json"), nil
	}
	// Legacy packages keep app/ and data/ as siblings. Signed updates do
	// not change their existing marker or data location.
	appDir := filepath.Dir(exe)
	if goos == "darwin" {
		if filepath.Base(appDir) != "MacOS" || filepath.Base(filepath.Dir(appDir)) != "Contents" {
			return "", errors.New("便携版必须从 app 目录中的应用包启动")
		}
		bundle := filepath.Dir(filepath.Dir(appDir))
		if !strings.HasSuffix(filepath.Base(bundle), ".app") {
			return "", errors.New("便携版应用包路径无效")
		}
		appDir = filepath.Dir(bundle)
	}
	if filepath.Base(appDir) != "app" {
		return "", errors.New("便携版必须从 app 目录启动")
	}
	return filepath.Join(filepath.Dir(appDir), "data", "state.json"), nil
}
