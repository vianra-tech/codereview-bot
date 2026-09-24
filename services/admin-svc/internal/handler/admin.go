package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vianra/codereview/admin-svc/internal/db"
	"go.uber.org/zap"
)

// AdminHandler handles management API requests
type AdminHandler struct {
	db     *db.DB
	logger *zap.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *db.DB, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		db:     db,
		logger: logger,
	}
}

// CreateOrganization handles POST /orgs
func (h *AdminHandler) CreateOrganization(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		Slug       string `json:"slug" binding:"required"`
		Provider   string `json:"provider" binding:"required"`
		ProviderID string `json:"provider_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := h.db.CreateOrganization(c.Request.Context(), req.Name, req.Slug, req.Provider, req.ProviderID)
	if err != nil {
		h.logger.Error("Failed to create org", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create organization"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// GetOrganization handles GET /orgs/:id
func (h *AdminHandler) GetOrganization(c *gin.Context) {
	id := c.Param("id")
	org, err := h.db.GetOrganization(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	c.JSON(http.StatusOK, org)
}

// AddRepository handles POST /orgs/:id/repos
func (h *AdminHandler) AddRepository(c *gin.Context) {
	orgID := c.Param("id")
	var req struct {
		Provider       string `json:"provider" binding:"required"`
		ProviderRepoID string `json:"provider_repo_id" binding:"required"`
		FullName       string `json:"full_name" binding:"required"`
		DefaultBranch  string `json:"default_branch"`
		Language       string `json:"language"`
		IsPrivate      bool   `json:"is_private"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.AddRepository(c.Request.Context(), orgID, req.Provider, req.ProviderRepoID, req.FullName, req.DefaultBranch, req.Language, req.IsPrivate); err != nil {
		h.logger.Error("Failed to add repo", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add repository"})
		return
	}

	c.Status(http.StatusCreated)
}

// GetRepositories handles GET /orgs/:id/repos
func (h *AdminHandler) GetRepositories(c *gin.Context) {
	orgID := c.Param("id")
	repos, err := h.db.GetOrganizationRepos(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch repositories"})
		return
	}

	c.JSON(http.StatusOK, repos)
}

// UpdateSubscription handles POST /orgs/:id/subscription
func (h *AdminHandler) UpdateSubscription(c *gin.Context) {
	orgID := c.Param("id")
	var req struct {
		Plan                 string `json:"plan" binding:"required"`
		StripeCustomerID     string `json:"stripe_customer_id" binding:"required"`
		StripeSubscriptionID string `json:"stripe_subscription_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateSubscription(c.Request.Context(), orgID, req.Plan, req.StripeCustomerID, req.StripeSubscriptionID); err != nil {
		h.logger.Error("Failed to update subscription", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "subscription updated"})
}
