package main

import (
	"log"
	"net/http"

	"github.com/gsarmaonline/gobot/internal/config"
	"github.com/gsarmaonline/gobot/internal/database"
	"github.com/gsarmaonline/gobot/internal/handlers"
	"github.com/gsarmaonline/gobot/internal/models"
	"github.com/gsarmaonline/gobot/internal/router"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations complete")

	// Initialize stores
	orgStore := models.NewOrganizationStore(db.DB)
	userStore := models.NewUserStore(db.DB)
	agentStore := models.NewAgentStore(db.DB)
	deptStore := models.NewDepartmentStore(db.DB)
	hierarchyStore := models.NewHierarchyStore(db.DB)

	_ = userStore // Will be used in later phases

	// Initialize handlers
	orgHandler := handlers.NewOrganizationHandler(orgStore)
	agentHandler := handlers.NewAgentHandler(agentStore)
	deptHandler := handlers.NewDepartmentHandler(deptStore)
	hierarchyHandler := handlers.NewHierarchyHandler(hierarchyStore)

	// Create router
	r := router.New(orgHandler, agentHandler, deptHandler, hierarchyHandler)

	// Start server
	addr := ":" + cfg.Port
	log.Printf("GoBot server starting on %s (env: %s)", addr, cfg.Env)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
