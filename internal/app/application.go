package app

import (
	"context"
	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/statework"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"github.com/tangqiu0205/LiuguangTV/internal/webplayer"
)

type application struct {
	window                                    *mygo.Window
	host                                      *embedhost.Host
	videoViewport                             ui.Rect // Last laid-out client-space video region.
	diagnostics                               *diagnostic.Logger
	diagnosticError                           error
	transferBusy                              bool
	configReplacing                           bool
	transferDone                              chan struct{}
	lastImportSnapshot                        string
	epgGeneration                             uint64
	state                                     store.State
	configError                               error // Startup read failure blocks persistence for this session.
	path                                      string
	version                                   string
	source, query, group, status, now, nowURL string
	showSettings                              bool
	settingsEPGTab                            bool
	settingsPlayerTab                         bool
	settingsAppearanceTab                     bool
	settingsBackupTab                         bool
	settingsAboutTab                          bool
	aboutScroll                               ui.ScrollState
	playerCompatibilityOpen                   bool
	backupDetailsOpen                         bool
	playerPathDraft                           string
	playerPathDraftLoaded                     bool
	appearanceScroll                          ui.ScrollState
	languagePreference                        string // UI cache of MyGo\'s saved override; empty follows system.
	backupScroll                              ui.ScrollState
	showSubEditor                             bool
	showSearch                                bool
	subName, editingID, deletingID            string
	subEPG                                    bool
	editingEPGID, deletingEPGID               string
	refreshing                                map[string]uint64
	refreshResult                             subscriptionRefreshResult
	cancelLoads                               map[string]context.CancelFunc
	loadSlots                                 chan struct{}
	loadSequence                              uint64
	favoritesOnly, busy, closed, cinema       bool
	windowCinema                              bool
	fullscreenUI                              fullscreenControls
	fullscreenPointer                         func() (int32, int32, bool, error)
	fullscreenControlsHovered                 bool
	list                                      ui.ListState
	settingsScroll                            ui.ScrollState
	epgSettingsScroll                         ui.ScrollState
	playerSettingsScroll                      ui.ScrollState
	editorScroll                              ui.ScrollState
	webView                                   *mygo.WebView
	webPlayer                                 *webplayer.Player
	player                                    playbackEngine
	activeChannel                             playlist.Channel
	activeSourceID                            string
	playbackCatalogBound                      bool
	triedLines                                map[string]bool
	lineIndex                                 int
	selectedLine                              string
	lineReady                                 bool
	startupTimer                              *time.Timer
	stallTimer                                *time.Timer
	settingsToastTimer                        *time.Timer
	settingsToastGeneration                   uint64
	settingsToast                             string
	pendingSettingsSuccess                    string
	settingsErrors                            map[string]string
	saveError                                 string
	lastProgress                              time.Time
	lastPosition                              float64
	hasPosition, playerPaused, buffering      bool
	dispatch                                  func(func())
	controls                                  player.Controls
	generation, settle                        int
	background                                *statework.Worker
	stateSequence                             uint64
	catalogDirty                              bool
	channelCache                              channelViewCache
	favoritesRevision                         uint64
	epg                                       epgState
	autoRefresh                               bool
	nextSubscriptionCheck                     time.Time
}

func (a *application) update(fn func()) {
	post := a.dispatch
	if post == nil {
		post = a.window.Update
	}
	post(func() {
		if !a.closed {
			fn()
		}
	})
}
func (a *application) toggleWindowCinema() {
	a.fullscreenUI = fullscreenControls{}
	a.fullscreenControlsHovered = false
	if a.cinema {
		a.cinema = false
		a.windowCinema = true
		if a.window != nil {
			a.window.SetFullScreen(false)
		}
	} else {
		a.windowCinema = !a.windowCinema
	}
	a.settle = 3
}
func (a *application) toggleCinema() {
	a.cinema = !a.cinema
	a.fullscreenUI = fullscreenControls{}
	a.fullscreenControlsHovered = false
	a.settle = 3
	if a.window != nil {
		a.window.SetFullScreen(a.cinema)
	}
}
func (a *application) cleanup() {
	if a.closed {
		return
	}
	a.closed = true
	if a.transferDone != nil {
		<-a.transferDone
	}
	for _, cancel := range a.cancelLoads {
		cancel()
	}
	for _, job := range a.epg.jobs {
		if job.cancel != nil {
			job.cancel()
		}
	}
	if a.epg.calendarCancel != nil {
		a.epg.calendarCancel()
	}
	if a.epg.cancelCleanup != nil {
		a.epg.cancelCleanup()
	}
	a.cancelStartupTimer()
	a.cancelStallTimer()
	if a.settingsToastTimer != nil {
		a.settingsToastTimer.Stop()
	}
	if a.player != nil {
		a.player.Stop()
	}
	a.releaseWebPlayback()
	a.host.Close()
	if a.background != nil {
		if err := a.background.Close(); err != nil {
			a.diagnostics.Record(diagnostic.StateSave, "", err)
			log.Print(tr("保存设置失败，请检查配置目录及诊断日志"))
		}
	}
}
func (a *application) versionLabel() string {
	if a.version == "" {
		return "dev"
	}
	return a.version
}
