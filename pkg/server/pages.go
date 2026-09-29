package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
)

//============================================================================
// Application Pages
//============================================================================

func (s *Server) Dashboard(c *gin.Context) {
	ctx := scene.New(c).ForPage("dashboard")
	c.HTML(http.StatusOK, "app/dashboard/index.html", ctx)
}
