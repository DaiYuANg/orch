package ingress

import (
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type liveRoutes struct {
	compiled []fiberRoute
}

type fiberRoute struct {
	meta routeMeta
	h    fiber.Handler
}

func newIngressFiberApp(log *slog.Logger, live *atomic.Pointer[liveRoutes]) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           60 * time.Second,
		WriteTimeout:          60 * time.Second,
		IdleTimeout:           120 * time.Second,
	})
	app.Use(ingressAccessLogFiber(log))

	app.Use(func(c *fiber.Ctx) error {
		lr := live.Load()
		if lr == nil || len(lr.compiled) == 0 {
			c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
			return c.Status(fiber.StatusOK).SendString("orch ingress is running")
		}
		p := normalizedPath(c.Path())
		for i := range lr.compiled {
			if lr.compiled[i].meta.matches(p) {
				return lr.compiled[i].h(c)
			}
		}
		return fiber.ErrNotFound
	})

	return app, nil
}

func buildFiberRoutes(routes []config.IngressRoute) ([]fiberRoute, error) {
	if len(routes) == 0 {
		return nil, nil
	}
	out := make([]fiberRoute, 0, len(routes))
	for i := range routes {
		raw := &routes[i]
		meta, err := newRouteMeta(raw)
		if err != nil {
			return nil, oopsx.B("ingress").Wrapf(err, "route %d (prefix %q)", i, raw.PathPrefix)
		}
		eps := raw.UpstreamEndpoints()
		if len(eps) > 1 {
			if pol := raw.LBPolicy(); pol != "round_robin" {
				return nil, oopsx.B("ingress").Errorf("route %d: unsupported ingress.lb %q (supported: round_robin)", i, raw.LB)
			}
		}
		servers := normalizeProxyServers(eps)
		if len(servers) == 0 {
			return nil, oopsx.B("ingress").Errorf("route %d: no valid upstream URLs", i)
		}
		metaCopy := meta
		h := proxy.Balancer(proxy.Config{
			Servers: servers,
			Timeout: 60 * time.Second,
			ModifyRequest: func(c *fiber.Ctx) error {
				p := normalizedPath(c.Path())
				if !metaCopy.matches(p) {
					return fiber.ErrNotFound
				}
				rel, ok := metaCopy.pathRel(p)
				if !ok {
					return fiber.ErrNotFound
				}
				c.Request().URI().SetPath(rel)
				return nil
			},
		})
		out = append(out, fiberRoute{meta: meta, h: h})
	}
	return out, nil
}

func normalizeProxyServers(eps []string) []string {
	out := make([]string, 0, len(eps))
	for _, s := range eps {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
			s = "http://" + s
		}
		out = append(out, s)
	}
	return out
}

func ingressAccessLogFiber(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		code := c.Response().StatusCode()
		if err != nil {
			var fe *fiber.Error
			if errors.As(err, &fe) {
				code = fe.Code
			}
		}
		log.Info("handled request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", code),
			slog.String("remote_addr", c.IP()),
			slog.Duration("duration", time.Since(start)),
		)
		return err
	}
}
