package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/gig/list_gigs.ssac contract=c8ddeb3
func (h *Handler) ListGigs(c *gin.Context) {
	opts := model.ParseQueryOpts(c, model.QueryOptsConfig{
		Pagination: &model.PaginationConfig{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
		Sort:       &model.SortConfig{Allowed: []string{"budget", "created_at"}, Default: "created_at", Direction: "desc"},
		Filter:     &model.FilterConfig{Allowed: []string{"status", "budget"}},
	})

	gigPage, err := h.GigModel.List(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gigPage)

}
