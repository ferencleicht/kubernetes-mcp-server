package main

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ferencleicht/kubernetes-mcp-server/pkg/kubernetes"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	gok8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var version = "0.1.0"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "server",
		Short:   "Kubernetes MCP Server",
		Long:    "A server that provides a MCP interface to Kubernetes",
		Version: fmt.Sprintf("Version %s\n, Build date %s", version, date),
	}

	sseCmd = &cobra.Command{
		Use:   "sse",
		Short: "Run the server in SSE mode",
		Long:  "Start a server that communicates via Server-Sent Events (SSE) using JSON-RPC messages.",
		Run: runServer(runSSEServer),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Run the server in stdio mode",
		Long:  "Start a server that communicates via standard input/output streams using JSON-RPC messages.",
		Run: runServer(runStdioServer),
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	rootCmd.PersistentFlags().StringSlice("toolsets", kubernetes.DefaultToolsets, "Comma-separated list of toolsets to enable")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to the log file")

	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))

	rootCmd.AddCommand(stdioCmd)
	rootCmd.AddCommand(sseCmd)
}

func initConfig() {
	viper.SetEnvPrefix("k8s")
	viper.AutomaticEnv()
}

func initLogger(outPath string) (*log.Logger, error) {
	logger := log.New()
	if outPath != "" {
		file, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.SetOutput(file)
	}

	logger.SetLevel(log.DebugLevel)

	return logger, nil
}

type runConfig struct {
	readOnly bool
	logger   *log.Logger
	enabledToolsets []string
}

func createKubernetesClient() (*gok8s.Clientset, error) {
	kubeconfig := viper.GetString("kubeconfig")
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := gok8s.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

func runServer(runFunc func(cfg runConfig, ctx context.Context) error) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		logFile := viper.GetString("log-file")
		readOnly := viper.GetBool("read-only")

		logger, err := initLogger(logFile)
		if err != nil {
			stdlog.Fatal("Failed to initialize logger:", err)
		}

		var enabledToolsets []string
		err = viper.UnmarshalKey("toolsets", &enabledToolsets)
		if err != nil {
			stdlog.Fatal("Failed to unmarshal toolsets:", err)
		}

		cfg := runConfig{
			readOnly: readOnly,
			logger:   logger,
			enabledToolsets: enabledToolsets,
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		if err := runFunc(cfg, ctx); err != nil {
			stdlog.Fatal("Failed to run server:", err)
		}
	}
}

func runStdioServer(cfg runConfig, ctx context.Context) error {
	clientset, err := createKubernetesClient()
	if err != nil {
		return err
	}

	getClient := func(_ context.Context) (*gok8s.Clientset, error) {
		return clientset, nil
	}

	k8sserver := kubernetes.NewServer(version)

	toolsets, err := kubernetes.InitToolsets(cfg.enabledToolsets, cfg.readOnly, getClient)
	if err != nil {
		return fmt.Errorf("failed to initialize toolsets: %w", err)
	}

	toolsets.RegisterTools(k8sserver)

	stdioServer := server.NewStdioServer(k8sserver)
	stdLogger := stdlog.New(cfg.logger.Writer(), "stdioserver", 0)
	stdioServer.SetErrorLogger(stdLogger)

	errC := make(chan error, 1)
	go func() {
		in, out := io.Reader(os.Stdin), io.Writer(os.Stdout)
		errC <- stdioServer.Listen(ctx, in, out)
	}()

	_, _ = fmt.Fprintf(os.Stderr, "Kubernetes MCP Server is running on stdio\n")

	select {
	case <-ctx.Done():
		cfg.logger.Infof("Received termination signal, shutting down...")
	case err := <-errC:
		if err != nil {
			return fmt.Errorf("error in server: %w", err)
		}
	}

	return nil
}

func runSSEServer(cfg runConfig, ctx context.Context) error {
	clientset, err := createKubernetesClient()
	if err != nil {
		return err
	}

	getClient := func(_ context.Context) (*gok8s.Clientset, error) {
		return clientset, nil
	}

	k8sserver := kubernetes.NewServer(version)

	toolsets, err := kubernetes.InitToolsets(cfg.enabledToolsets, cfg.readOnly, getClient)
	if err != nil {
		return fmt.Errorf("failed to initialize toolsets: %w", err)
	}

	toolsets.RegisterTools(k8sserver)

	sseServer := server.NewSSEServer(k8sserver, server.WithBaseURL("http://localhost:8080"))

	errC := make(chan error, 1)
	go func() {
		errC <- sseServer.Start(":8080")
	}()

	_, _ = fmt.Fprintf(os.Stderr, "Kubernetes MCP Server is running on SSE\n")

	select {
	case <-ctx.Done():
		cfg.logger.Infof("Received termination signal, shutting down...")
	case err := <-errC:
		if err != nil {
			return fmt.Errorf("error in server: %w", err)
		}
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
