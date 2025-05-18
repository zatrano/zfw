package middlewares

import (
	"zatrano/configs/sessionconfig"
	"zatrano/pkg/flashmessages"
	"zatrano/services"

	"github.com/gofiber/fiber/v2"
)

func StatusMiddleware(c *fiber.Ctx) error {
	userID, err := sessionconfig.GetUserIDFromSession(c)
	if err != nil || userID == 0 {
		return c.Redirect("/auth/login")
	}

	authService := services.NewAuthService()
	user, err := authService.GetUserProfile(userID)
	if err != nil {
		sessionconfig.DestroySession(c)
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Kullanıcı bulunamadı")
		return c.Redirect("/auth/login")
	}

	if !user.Status {
		sessionconfig.DestroySession(c)
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Kullanıcı durumu geçersiz")
		return c.Redirect("/auth/login")
	}

	return c.Next()
}
