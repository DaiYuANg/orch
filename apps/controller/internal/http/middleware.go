package http

import (
	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var middleware = fx.Module("middleware",
	fx.Invoke(
		registerFavicon,
		registerMonitor,
		registerPrometheus,
		registerHealthcheck,
		registerRequestId,
		registerPprof,
		registerCompress,
		registerLogger,
		websocketUpgrader,
	),
)

func registerMonitor(app *fiber.App) {
	app.Get("/metrics", monitor.New())
}

func registerPrometheus(app *fiber.App) {
	prometheus := fiberprometheus.New("my-service-name")
	prometheus.RegisterAt(app, "/metrics")
	prometheus.SetSkipPaths([]string{"/ping"})            // Optional: Remove some paths from metrics
	prometheus.SetIgnoreStatusCodes([]int{401, 403, 404}) // Optional: Skip metrics for these status codes
	app.Use(prometheus.Middleware)
}

func registerHealthcheck(app *fiber.App) {
	app.Use(healthcheck.New(healthcheck.ConfigDefault))
}

func registerRequestId(app *fiber.App) {
	app.Use(requestid.New())
}

func registerPprof(app *fiber.App) {
	app.Use(pprof.New())
}

func registerCompress(app *fiber.App) {
	app.Use(compress.New())
}

func registerLogger(app *fiber.App, logger *zap.Logger) {
	app.Use(fiberzap.New(fiberzap.Config{
		Logger: logger,
	}))
}

func registerFavicon(app *fiber.App) {
	app.Use(favicon.New())
}

func websocketUpgrader(app *fiber.App) {
	app.Use(func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
}
