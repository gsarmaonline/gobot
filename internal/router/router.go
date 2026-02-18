package router

import (
	"net/http"

	"github.com/gsarmaonline/gobot/internal/handlers"
)

func New(
	orgHandler *handlers.OrganizationHandler,
	agentHandler *handlers.AgentHandler,
	deptHandler *handlers.DepartmentHandler,
	hierarchyHandler *handlers.HierarchyHandler,
) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/v1/health", handlers.HealthCheck)

	// Organizations
	mux.HandleFunc("POST /api/v1/organizations", orgHandler.Create)
	mux.HandleFunc("GET /api/v1/organizations", orgHandler.List)
	mux.HandleFunc("GET /api/v1/organizations/{id}", orgHandler.Get)

	// Agents
	mux.HandleFunc("POST /api/v1/agents", agentHandler.Create)
	mux.HandleFunc("GET /api/v1/agents", agentHandler.List)
	mux.HandleFunc("GET /api/v1/agents/{id}", agentHandler.Get)
	mux.HandleFunc("PUT /api/v1/agents/{id}", agentHandler.Update)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", agentHandler.Delete)
	mux.HandleFunc("POST /api/v1/agents/{id}/{action}", agentHandler.UpdateStatus)

	// Departments
	mux.HandleFunc("POST /api/v1/departments", deptHandler.Create)
	mux.HandleFunc("GET /api/v1/departments", deptHandler.List)
	mux.HandleFunc("GET /api/v1/departments/{id}", deptHandler.Get)
	mux.HandleFunc("PUT /api/v1/departments/{id}", deptHandler.Update)
	mux.HandleFunc("DELETE /api/v1/departments/{id}", deptHandler.Delete)

	// Hierarchy
	mux.HandleFunc("POST /api/v1/hierarchy", hierarchyHandler.Create)
	mux.HandleFunc("GET /api/v1/hierarchy", hierarchyHandler.List)
	mux.HandleFunc("GET /api/v1/hierarchy/tree", hierarchyHandler.GetTree)
	mux.HandleFunc("DELETE /api/v1/hierarchy/{id}", hierarchyHandler.Delete)

	return withCORS(withLogging(mux))
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
