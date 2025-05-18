package routes

import (
	handlers "zatrano/handlers/website"

	"github.com/gofiber/fiber/v2"
)

func registerWebsiteRoutes(app *fiber.App) {
	app.Get("/", handlers.NewWebsiteHandler().ShowHomePage)
}
