package api

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true

	return
}

// RequestLoggingMiddleware is middleware that logs requests
func RequestLoggingMiddleware(l *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					rw.WriteHeader(http.StatusInternalServerError)

					if err, ok := err.(error); ok {
						l.Error("Error completing response",
							zap.Error(err),
							zap.String("method", r.Method),
							zap.String("path", r.URL.EscapedPath()),
						)
					}

				}
			}()

			start := time.Now()
			wrapped := wrapResponseWriter(rw)
			next.ServeHTTP(wrapped, r)
			l.Info("Response complete",
				zap.Int("status", wrapped.Status()),
				zap.String("method", r.Method),
				zap.String("path", r.URL.EscapedPath()),
				zap.String("duration", fmt.Sprintf("%fms", timeInMilliSeconds(time.Since(start)))),
				zap.String("user_agent", r.UserAgent()),
			)
		})
	}
}

func timeInMilliSeconds(duration time.Duration) float64 {
	seconds := float64(duration / time.Millisecond)
	microSeconds := float64(duration % time.Millisecond)
	return seconds + microSeconds/1e6
}
