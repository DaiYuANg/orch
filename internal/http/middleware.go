package http

import (
	"io"
	"log/slog"
	"os"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"go.uber.org/fx"
)

var middleware = fx.Module("middleware",
	fx.Invoke(
		registerFavicon,
		registerPrometheus,
		registerHealthcheck,
		registerRequestId,
		registerPprof,
		registerCompress,
		registerLogger,
		websocketUpgrader,
	),
)

func registerPrometheus(app *fiber.App) {
	prometheus := fiberprometheus.New("warden")
	prometheus.RegisterAt(app, "/metrics")
	prometheus.SetSkipPaths([]string{"/ping"})
	prometheus.SetIgnoreStatusCodes([]int{401, 403, 404})
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

type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (int, error) {
	w.logger.Info(string(p))
	return len(p), nil
}

func registerLogger(app *fiber.App, logger *slog.Logger) {
	writer := io.Writer(&slogWriter{logger: logger})
	app.Use(fiberlogger.New(fiberlogger.Config{
		Output: writer,
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
}

func registerFavicon(app *fiber.App) {
	app.Use(favicon.New())
}

func websocketUpgrader(app *fiber.App) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
}
