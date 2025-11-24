package http

import (
	"gomobile/internal/service/logic"

	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	engine *gin.Engine
	logic  *logic.BusinessLogic
}

func NewHTTPServer(logic *logic.BusinessLogic) *HTTPServer {
	r := gin.New()
	r.Use(gin.Recovery())

	srv := &HTTPServer{
		engine: r,
		logic:  logic,
	}

	srv.initMiddleware()
	srv.initRoutes()

	return srv
}

func (h *HTTPServer) Start(port string) error {
	return h.engine.Run(":" + port)
}
