package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	assetRouter := router.Group("/v1/assets")
	assetRouter.Use(middleware.RouteTag("relay"), middleware.TokenAuth())
	{
		assetRouter.POST("", controller.CreateStarAIAsset)
		assetRouter.GET("/:id", controller.GetStarAIAsset)
		assetRouter.DELETE("/:id", controller.DeleteStarAIAsset)
	}
	videoSharedRouter := router.Group("/v1")
	videoSharedRouter.Use(middleware.RouteTag("relay"))
	videoSharedRouter.Use(middleware.TokenAuth())
	videoSharedRouter.Use(middleware.SystemPerformanceCheck())
	videoSharedRouter.POST(
		"/video/generations",
		middleware.PinTaskPluginEndpoint(),
		middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()),
		middleware.PrepareTaskPluginEndpoint(),
		middleware.Distribute(),
		func(c *gin.Context) { controller.RelayTaskPluginEndpoint(c, controller.RelayTask) },
	)

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}
	// openai compatible API video routes
	// docs: https://platform.openai.com/docs/api-reference/videos/create
	{
		videoV1Router.POST("/videos/generations", controller.RelayTask)
		videoV1Router.POST("/videos/edits", controller.RelayTask)
		videoV1Router.POST("/videos/extensions", controller.RelayTask)
	}

}
