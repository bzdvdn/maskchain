package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
)

// @sk-task ui-production-readiness#T3.1: Server-side pagination/search for list endpoints (AC-008)
// @sk-task ui-production-readiness#T7.1: Handlers page through the repository (AC-008)
//
// List endpoints accept optional limit/offset/search. When none are supplied the
// previous behavior (return everything) is preserved; when a limit is supplied
// the handler asks the repository for just that page plus a total count.
const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

type listQuery struct {
	Active bool
	Limit  int
	Offset int
	Search string
}

func parseListQuery(c *gin.Context) listQuery {
	rawLimit := c.Query("limit")
	rawSearch := strings.TrimSpace(c.Query("search"))
	if rawLimit == "" && rawSearch == "" {
		return listQuery{}
	}

	limit := defaultPageLimit
	if rawLimit != "" {
		if n, err := strconv.Atoi(rawLimit); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}

	offset := 0
	if n, err := strconv.Atoi(c.Query("offset")); err == nil && n > 0 {
		offset = n
	}

	return listQuery{Active: true, Limit: limit, Offset: offset, Search: rawSearch}
}

func (q listQuery) page() int {
	return q.Offset/q.Limit + 1
}

// writePage writes the paginated envelope for an already-paged result set.
func writePage(c *gin.Context, data any, q listQuery, total int) {
	c.Set(middleware.EnvelopedKey, true)
	c.JSON(http.StatusOK, dto.NewSuccessPaginated(data, q.page(), q.Limit, total))
}
