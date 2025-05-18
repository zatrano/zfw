package handlers

import (
	"net/http"

	"zatrano/pkg/renderer"

	"github.com/gofiber/fiber/v2"
)

type WebsiteHandler struct {
}

func NewWebsiteHandler() *WebsiteHandler {
	return &WebsiteHandler{}
}

func (h *WebsiteHandler) ShowHomePage(c *fiber.Ctx) error {
	mapData := fiber.Map{}
	return renderer.Render(c, "website/home", "layouts/website", mapData, http.StatusOK)
}
