package appcmd

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/go-chi/chi/v5"
	httpx "github.com/yoke233/zhanggui/internal/adapters/http/server"
	"github.com/yoke233/zhanggui/internal/platform/bootstrap"
	"github.com/yoke233/zhanggui/internal/platform/config"
)

type HTTPRuntimeOptions struct {
	Command              string
	ListenPort           int
	WithSignalServerAddr bool
}

type HTTPRuntime struct {
	Config         *config.Config
	DataDir        string
	ServerPort     int
	AdminToken     string
	SkipAuth       bool
	TokenRegistry  *httpx.TokenRegistry
	RouteRegistrar func(chi.Router)
	cleanup        func()
}

func BootstrapHTTPRuntime(opts HTTPRuntimeOptions) (*HTTPRuntime, error) {
	commandName := strings.TrimSpace(opts.Command)
	if commandName == "" {
		commandName = "server"
	}

	cfg, dataDir, secrets, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	serverPort := resolveServerPort(opts.ListenPort, cfg.Server.Port)

	closeLog, err := InitAppLogger(dataDir, commandName)
	if err != nil {
		return nil, err
	}

	tokenRegistry := httpx.NewTokenRegistry(secrets.Tokens)
	serverAddr := ""
	if opts.WithSignalServerAddr {
		serverAddr = buildServerBaseURL(cfg.Server.Host, serverPort)
	}
	signalCfg := &bootstrap.AgentSignalConfig{
		TokenRegistry: tokenRegistry,
		ServerAddr:    serverAddr,
	}

	store, _, runtimeManager, cleanupFn, registrar := bootstrap.Build(
		ExpandStorePath(cfg.Store.Path, dataDir),
		nil,
		cfg,
		bootstrap.SCMTokens{
			GitHub: strings.TrimSpace(secrets.GitHub.PAT),
			Codeup: strings.TrimSpace(secrets.Codeup.PAT),
		},
		nil,
		signalCfg,
	)

	cleanup := func() {
		if runtimeManager != nil {
			_ = runtimeManager.Close()
		}
		if cleanupFn != nil {
			cleanupFn()
		}
		_ = closeLog()
	}

	if store == nil || registrar == nil {
		cleanup()
		return nil, fmt.Errorf("bootstrap %s failed", commandName)
	}

	return &HTTPRuntime{
		Config:         cfg,
		DataDir:        dataDir,
		ServerPort:     serverPort,
		AdminToken:     secrets.AdminToken(),
		SkipAuth:       !cfg.Server.IsAuthRequired(),
		TokenRegistry:  tokenRegistry,
		RouteRegistrar: registrar,
		cleanup:        cleanup,
	}, nil
}

func (r *HTTPRuntime) Close() {
	if r == nil || r.cleanup == nil {
		return
	}
	r.cleanup()
	r.cleanup = nil
}

func (r *HTTPRuntime) NewServer(addr string, frontend fs.FS, apiOnly bool) *httpx.Server {
	return httpx.NewServer(httpx.Config{
		Addr:           addr,
		Auth:           r.TokenRegistry,
		Frontend:       frontend,
		RouteRegistrar: r.RouteRegistrar,
		SkipAuth:       r.SkipAuth,
		APIOnly:        apiOnly,
	})
}
