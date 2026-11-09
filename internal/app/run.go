// Package app owns the desktop UI and coordinates the domain services.
// Start it from the process's initial OS thread through Run.
package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func resolveBuildVersion(buildVersion, packagedVersion string) (string, error) {
	if buildVersion == "" || buildVersion == "dev" {
		if packagedVersion != "" {
			return packagedVersion, nil
		}
		return "dev", nil
	}
	if packagedVersion != "" && packagedVersion != buildVersion {
		return "", fmt.Errorf(tr("构建版本 %s 与应用包版本 %s 不一致"), buildVersion, packagedVersion)
	}
	return buildVersion, nil
}

// Run starts the native application. buildVersion is supplied by the command
// entry point so the application package does not depend on package main.
func Run(buildVersion string) error {
	path, portable, err := store.Path()
	if err != nil {
		return err
	}
	lock, err := store.LockData(path)
	if err != nil {
		return err
	}
	defer lock.Close()
	buildVersion, err = resolveBuildVersion(buildVersion, mygo.App.Version())
	if err != nil {
		return err
	}
	mygo.App.SetPath(mygo.PathUserData, filepath.Dir(path))
	if portable {
		mygo.App.SetPath(mygo.PathCache, filepath.Join(filepath.Dir(path), "cache"))
		mygo.App.SetPath(mygo.PathLogs, filepath.Join(filepath.Dir(path), "logs"))
	}
	mygo.App.SetVersion(buildVersion)
	mygo.App.SetLanguages("zh-Hans", "en")
	mygo.Use(native.New(updater.Options{}))
	state, err := store.Load(path)
	status := tr("导入节目单开始观看；视频在右侧区域内嵌播放")
	if len(state.Channels) > 0 {
		status = tr("已加载缓存；订阅源与 EPG 每 24 小时自动更新一次")
	}
	if !embedhost.Supported {
		status = tr("此平台尚未适配内嵌播放，使用独立 mpv 窗口")
	}
	if err != nil {
		status = tr("配置读取或恢复失败，请检查并备份配置文件：") + err.Error()
	}
	var recovery *store.RecoveryNotice
	canRefresh := err == nil || errors.As(err, &recovery)
	if recovery != nil {
		status = recovery.Error()
	}
	a := &application{version: buildVersion, state: state, path: path, group: "全部分组", status: status, controls: player.Controls{Volume: 70}, settle: 3}
	a.diagnostics, a.diagnosticError = diagnostic.Open(filepath.Join(filepath.Dir(path), "logs"))
	a.diagnostics.Record(diagnostic.AppStarted, "", nil)
	if !canRefresh {
		a.configError = err
	}
	a.startBackground()
	mygo.Theme.SetSource(nativeThemeSource(state.ThemeMode))
	mygo.App.WhenReady(func() {
		a.refreshLanguagePreference()
		a.window = mygo.NewWindow(a.mainWindowOptions())
		a.window.OnReadyToShow(a.showPreparedWindow)
		a.host, err = embedhost.New(a.window.NativeHandle())
		if err != nil {
			a.status = err.Error()
		}
		a.window.OnResize(func() { a.settle = 3; a.window.Invalidate() })
		a.window.OnMove(func() { a.settle = 3; a.window.Invalidate() })
		a.window.OnLeaveFullScreen(func() { a.cinema = false; a.settle = 3; a.window.Invalidate() })
		a.window.OnClose(func(*mygo.CloseEvent) { a.cleanup() })
		a.window.OnClosed(func() { mygo.App.Quit() })
		a.window.Invalidate()
		prepareInitialFrame(a.window)
		if canRefresh {
			a.autoRefresh = true
			a.syncEPG(false)
			a.pollSubscriptions()
		}
	})
	mygo.App.OnLocaleChanged(a.localeChanged)
	mygo.App.OnQuit(a.cleanup)
	return mygo.App.Run()
}
