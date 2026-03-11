package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h *Handler) ListGigs(c *gin.Context) {
	opts := model.ParseQueryOpts(c, model.QueryOptsConfig{Pagination: &model.PaginationConfig{Style: "offset", DefaultLimit: 20, MaxLimit: 100}, Sort: &model.SortConfig{Allowed: []string{"budget", "created_at"}, Default: "created_at", Direction: "desc"}, Filter: &model.FilterConfig{Allowed: []string{"status", "budget"}}})
	if v := c.Query("limit"); v != "" {
		opts.Limit, _ = strconv.Atoi(v)
	}
	if v := c.Query("offset"); v != "" {
		opts.Offset, _ = strconv.Atoi(v)
	}
	if v := c.Query("sort"); v != "" {
		opts.SortCol = v
	}

	gigPage, err := h.GigModel.List(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gigPage)

}
