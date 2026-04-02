package desktopapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/yoke233/zhanggui/internal/platform/appcmd"
)

// Bootstrap contains the information the frontend needs to connect.
type Bootstrap struct {
	Token      string `json:"token"`
	APIBaseURL string `json:"apiBaseUrl"`
	WSBaseURL  string `json:"wsBaseUrl"`
}

// App manages the Go backend lifecycle within the Wails desktop shell.
type App struct {
	ctx        context.Context
	apiHandler http.Handler
	httpServer *http.Server
	token      string
	apiBaseURL string
	wsBaseURL  string
	runtime    *appcmd.HTTPRuntime
}

func New() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("desktop: listen loopback: %v", err)
	}
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		log.Fatalf("desktop: unexpected listener addr type %T", listener.Addr())
	}

	runtimeHost, err := appcmd.BootstrapHTTPRuntime(appcmd.HTTPRuntimeOptions{
		Command:              "desktop",
		ListenPort:           addr.Port,
		WithSignalServerAddr: true,
	})
	if err != nil {
		_ = listener.Close()
		log.Fatalf("desktop: bootstrap runtime: %v", err)
	}

	a.runtime = runtimeHost
	a.token = runtimeHost.AdminToken

	srv := runtimeHost.NewServer("", nil, true)
	a.apiHandler = srv.Handler()
	a.httpServer = &http.Server{Handler: a.apiHandler}

	go func() {
		if serveErr := a.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Printf("desktop: api server stopped: %v", serveErr)
		}
	}()

	baseURL := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", addr.Port)),
		Path:   "/api",
	}
	a.apiBaseURL = baseURL.String()
	a.wsBaseURL = a.apiBaseURL
}

func (a *App) Shutdown(_ context.Context) {
	if a.httpServer != nil {
		_ = a.httpServer.Close()
		a.httpServer = nil
	}
	if a.runtime != nil {
		a.runtime.Close()
		a.runtime = nil
	}
}

// GetBootstrap returns the desktop bootstrap info (token) to the frontend.
func (a *App) GetBootstrap() Bootstrap {
	return Bootstrap{
		Token:      a.token,
		APIBaseURL: a.apiBaseURL,
		WSBaseURL:  a.wsBaseURL,
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
