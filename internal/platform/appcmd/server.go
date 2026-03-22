package appcmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func RunServer(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Println("[startup] parsing server args")
	port, err := parseServerPort(args)
	if err != nil {
		return err
	}
	fmt.Println("[startup] building runtime")
	runtimeHost, err := BootstrapHTTPRuntime(HTTPRuntimeOptions{
		Command:              "server",
		ListenPort:           port,
		WithSignalServerAddr: true,
	})
	if err != nil {
		return err
	}
	defer runtimeHost.Close()
	fmt.Printf("[startup] data dir: %s\n", runtimeHost.DataDir)
	listenAddr := buildServerAddress(runtimeHost.Config.Server.Host, runtimeHost.ServerPort)
	fmt.Println("[startup] resolving frontend assets")
	frontendFS, err := ResolveFrontendFS()
	if err != nil {
		return err
	}
	fmt.Println("[startup] creating http server")
	srv := runtimeHost.NewServer(listenAddr, frontendFS, false)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	fmt.Printf("Server started on %s (api: /api).\n", listenAddr)
	if runtimeHost.SkipAuth {
		fmt.Println("Auth: disabled (auth_required = false).")
	} else if runtimeHost.AdminToken != "" {
		fmt.Printf("Admin token: %s\n", runtimeHost.AdminToken)
	}
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}

func parseServerPort(args []string) (int, error) {
	port := 0
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--port":
			i++
			if i >= len(args) {
				return 0, fmt.Errorf("usage: ai-flow server [--port <port>]")
			}
			parsed, err := parsePortValue(args[i])
			if err != nil {
				return 0, err
			}
			port = parsed
		case strings.HasPrefix(arg, "--port="):
			parsed, err := parsePortValue(strings.TrimPrefix(arg, "--port="))
			if err != nil {
				return 0, err
			}
			port = parsed
		default:
			return 0, fmt.Errorf("usage: ai-flow server [--port <port>]")
		}
	}
	return port, nil
}

func parsePortValue(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid --port value: %s", raw)
	}
	return port, nil
}

func resolveServerPort(cliPort int, cfgPort int) int {
	if cliPort > 0 {
		return cliPort
	}
	if cfgPort > 0 && cfgPort <= 65535 {
		return cfgPort
	}
	return DefaultServerPort
}

func buildServerAddress(host string, port int) string {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return fmt.Sprintf(":%d", port)
	}
	return net.JoinHostPort(trimmedHost, strconv.Itoa(port))
}

func buildServerBaseURL(host string, port int) string {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		trimmedHost = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(trimmedHost, strconv.Itoa(port))
}
