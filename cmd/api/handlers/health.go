package handlers

import (
	"context"
	"net/http"
	"time"
)

// HealthCheck represents the status of a single health check.
type HealthCheck struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// HealthResponse represents the overall health response.
type HealthResponse struct {
	Status string                 `json:"status"`
	Checks map[string]HealthCheck `json:"checks"`
}

// Health returns a simple health check response (liveness probe).
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

// Ready checks if all dependencies are ready (readiness probe).
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status: "ok",
		Checks: make(map[string]HealthCheck),
	}

	// Check database
	dbCheck := h.checkDatabase(ctx)

	response.Checks["database"] = dbCheck
	if dbCheck.Status != "ok" {
		response.Status = "degraded"
	}

	// Check Redis
	redisCheck := h.checkRedis(ctx)

	response.Checks["redis"] = redisCheck
	if redisCheck.Status != "ok" {
		response.Status = "degraded"
	}

	// Check Zoekt (optional - don't fail if unavailable)
	zoektCheck := h.checkZoekt(ctx)
	response.Checks["zoekt"] = zoektCheck

	// Set HTTP status based on overall health
	statusCode := http.StatusOK
	if response.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSONWithStatus(w, statusCode, response)
}

func (h *Handler) checkDatabase(ctx context.Context) HealthCheck {
	start := time.Now()
	err := h.services.Pool.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return HealthCheck{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return HealthCheck{
		Status:  "ok",
		Latency: latency.String(),
	}
}

func (h *Handler) checkRedis(ctx context.Context) HealthCheck {
	start := time.Now()
	_, err := h.services.Redis.Ping(ctx).Result()
	latency := time.Since(start)

	if err != nil {
		return HealthCheck{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return HealthCheck{
		Status:  "ok",
		Latency: latency.String(),
	}
}

func (h *Handler) checkZoekt(ctx context.Context) HealthCheck {
	start := time.Now()
	err := h.services.Search.Health(ctx)
	latency := time.Since(start)

	if err != nil {
		return HealthCheck{
			Status:  "warning",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return HealthCheck{
		Status:  "ok",
		Latency: latency.String(),
	}
}
