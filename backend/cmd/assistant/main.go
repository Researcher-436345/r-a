// Command assistant runs the Article QA Assistant HTTP service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"assistant/internal/article"
	"assistant/internal/chat"
	"assistant/internal/config"
	"assistant/internal/httpapi"
	"assistant/internal/llm"
	"assistant/internal/qa"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// HTTP client for upstream (Article/Chat) calls.
	upstreamClient := &http.Client{Timeout: cfg.UpstreamTimeout}

	articleClient := article.New(cfg.ArticleServiceURL, upstreamClient)
	chatClient := chat.New(cfg.ChatServiceURL, upstreamClient)
	llmClient := llm.NewClient(llm.Options{
		APIKey:      cfg.OpenAIAPIKey,
		BaseURL:     cfg.OpenAIBaseURL,
		Model:       cfg.OpenAIModel,
		MaxTokens:   cfg.LLMMaxTokens,
		Temperature: cfg.LLMTemperature,
	})

	svc := qa.New(articleClient, chatClient, llmClient, cfg.UpstreamTimeout)
	router := httpapi.NewRouter(svc, logger, cfg.RequestTimeout)

	// WriteTimeout stays zero: SSE responses are long-lived and the per-request
	// budget is enforced via the context-timeout middleware instead.
	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr, "model", cfg.OpenAIModel)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
