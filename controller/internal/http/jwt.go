package http

import (
	"crypto/rsa"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
)

var jwt = fx.Module("jwt", fx.Invoke(jwtMiddleware))

func jwtMiddleware(app *fiber.App, privateKey *rsa.PrivateKey) {
	app.Use(jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{
			JWTAlg: jwtware.RS256,
			Key:    privateKey.Public(),
		},
	}))
}
