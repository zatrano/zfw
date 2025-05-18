package middlewares

import (
	"zatrano/configs/sessionconfig"

	"github.com/gofiber/fiber/v2"
)

func SessionMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := sessionconfig.SessionStart(c)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Session başlatılamadı")
		}
		c.Locals("session", sess)
		return c.Next()
	}
}
