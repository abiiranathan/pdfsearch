package routes

import (
	"io"
	"log/slog"
	"net/http"
	"time"
)

func Logger(out io.Writer) func(http.Handler) http.Handler {
	slogOpts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := slog.New(slog.NewTextHandler(out, slogOpts))

			start := time.Now()
			next.ServeHTTP(w, r)

			took := time.Since(start).String()
			logger.Info("", "latency", took, "method", r.Method, "path", r.URL.Path)
		})
	}
}
