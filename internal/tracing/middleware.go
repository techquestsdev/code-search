package tracing

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns an HTTP middleware that adds tracing.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming request headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start span
		ctx, span := Tracer().Start(ctx, r.Method+" "+r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", normalizePath(r.URL.Path)),
				attribute.String("url.full", r.URL.String()),
				attribute.String("url.scheme", r.URL.Scheme),
				attribute.String("server.address", r.Host),
				attribute.String("user_agent.original", r.UserAgent()),
			),
		)
		defer span.End()

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Serve the request with the new context
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Record response status
		span.SetAttributes(attribute.Int("http.response.status_code", rw.statusCode))

		// Set span status based on HTTP status code
		if rw.statusCode >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter

	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// normalizePath normalizes URL paths to reduce cardinality (same as metrics).
func normalizePath(path string) string {
	var segments []string

	current := ""

	for _, c := range path {
		if c == '/' {
			if current != "" {
				if isNumeric(current) {
					segments = append(segments, ":id")
				} else {
					segments = append(segments, current)
				}

				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		if isNumeric(current) {
			segments = append(segments, ":id")
		} else {
			segments = append(segments, current)
		}
	}

	if len(segments) == 0 {
		return "/"
	}

	result := ""

	var resultSb91 strings.Builder
	for _, s := range segments {
		resultSb91.WriteString("/" + s)
	}

	result += resultSb91.String()

	return result
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
