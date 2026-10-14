package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
)

//============================================================================
// Application Pages
//============================================================================

func (s *Server) Dashboard(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("dashboard")
	c.HTML(http.StatusOK, "dashboard/daybreak/list.html", data)
	return nil
}
