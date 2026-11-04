package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusOwnerNotificationHandler handles notification HTTP endpoints for bus owners
type BusOwnerNotificationHandler struct {
	notificationRepo *database.BusOwnerNotificationRepository
	busOwnerRepo     *database.BusOwnerRepository
}

// NewBusOwnerNotificationHandler creates a new BusOwnerNotificationHandler
func NewBusOwnerNotificationHandler(
	notificationRepo *database.BusOwnerNotificationRepository,
	busOwnerRepo *database.BusOwnerRepository,
) *BusOwnerNotificationHandler {
	return &BusOwnerNotificationHandler{
		notificationRepo: notificationRepo,
		busOwnerRepo:     busOwnerRepo,
	}
}

// GetNotifications returns paginated notifications for the authenticated bus owner
// GET /api/v1/bus-owner/notifications or GET /api/v1/notifications
func (h *BusOwnerNotificationHandler) GetNotifications(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	notifications, totalCount, err := h.notificationRepo.GetByOwnerID(busOwner.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications: " + err.Error()})
		return
	}

	unreadCount, err := h.notificationRepo.GetUnreadCount(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread count: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.NotificationListResponse{
		Notifications: notifications,
		UnreadCount:   unreadCount,
		TotalCount:    totalCount,
		Page:          page,
		Limit:         limit,
	})
}

// GetUnreadCount returns only the count of unread notifications for badge display
// GET /api/v1/bus-owner/notifications/unread-count or GET /api/v1/notifications/unread-count
func (h *BusOwnerNotificationHandler) GetUnreadCount(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	count, err := h.notificationRepo.GetUnreadCount(busOwner.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread count: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

// MarkAsRead marks a specific notification as read
// PATCH /api/v1/bus-owner/notifications/:id/read or PATCH /api/v1/notifications/:id/read
func (h *BusOwnerNotificationHandler) MarkAsRead(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	notificationID := c.Param("id")
	if notificationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Notification ID is required"})
		return
	}

	if _, err := uuid.Parse(notificationID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID format"})
		return
	}

	if err := h.notificationRepo.MarkAsRead(notificationID, busOwner.ID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

// MarkAllAsRead marks all unread notifications as read
// PATCH /api/v1/bus-owner/notifications/read-all or PATCH /api/v1/notifications/read-all
func (h *BusOwnerNotificationHandler) MarkAllAsRead(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	busOwner, err := h.busOwnerRepo.GetByUserID(userCtx.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bus owner profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}

	if err := h.notificationRepo.MarkAllAsRead(busOwner.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all as read: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}
