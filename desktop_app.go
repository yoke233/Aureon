package main

import (
	"context"
	"net/http"

	platformdesktop "github.com/yoke233/zhanggui/internal/platform/desktopapp"
)

type DesktopBootstrap = platformdesktop.Bootstrap

// DesktopApp keeps the Wails binding name stable while delegating implementation.
type DesktopApp struct {
	inner *platformdesktop.App
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{inner: platformdesktop.New()}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.inner.Startup(ctx)
}

func (a *DesktopApp) Shutdown(ctx context.Context) {
	a.inner.Shutdown(ctx)
}

func (a *DesktopApp) GetBootstrap() DesktopBootstrap {
	return a.inner.GetBootstrap()
}

func (a *DesktopApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.inner.ServeHTTP(w, r)
}
