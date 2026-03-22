package desktopapp

import (
	"context"
	"log"
	"net/http"

	"github.com/yoke233/zhanggui/internal/platform/appcmd"
)

// Bootstrap contains the information the frontend needs to connect.
type Bootstrap struct {
	Token string `json:"token"`
}

// App manages the Go backend lifecycle within the Wails desktop shell.
type App struct {
	ctx        context.Context
	apiHandler http.Handler
	token      string
	runtime    *appcmd.HTTPRuntime
}

func New() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	runtimeHost, err := appcmd.BootstrapHTTPRuntime(appcmd.HTTPRuntimeOptions{
		Command: "desktop",
	})
	if err != nil {
		log.Fatalf("desktop: bootstrap runtime: %v", err)
	}

	a.runtime = runtimeHost
	a.token = runtimeHost.AdminToken

	srv := runtimeHost.NewServer("", nil, true)
	a.apiHandler = srv.Handler()
}

func (a *App) Shutdown(_ context.Context) {
	if a.runtime != nil {
		a.runtime.Close()
		a.runtime = nil
	}
}

// GetBootstrap returns the desktop bootstrap info (token) to the frontend.
func (a *App) GetBootstrap() Bootstrap {
	return Bootstrap{
		Token: a.token,
	}
}

// ServeHTTP delegates API/WS requests from Wails AssetServer to the Go handler.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.apiHandler != nil {
		a.apiHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "server not ready", http.StatusServiceUnavailable)
	}
}
