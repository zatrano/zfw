package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"zatrano/configs/logconfig"
	"zatrano/models"
	"zatrano/repositories"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ServiceError string

func (e ServiceError) Error() string {
	return string(e)
}

const (
	ErrInvalidCredentials       ServiceError = "geçersiz kimlik bilgileri"
	ErrUserNotFound             ServiceError = "kullanıcı bulunamadı"
	ErrUserInactive             ServiceError = "kullanıcı aktif değil"
	ErrCurrentPasswordIncorrect ServiceError = "mevcut şifre hatalı"
	ErrPasswordTooShort         ServiceError = "yeni şifre en az 6 karakter olmalıdır"
	ErrPasswordSameAsOld        ServiceError = "yeni şifre mevcut şifre ile aynı olamaz"
	ErrAuthGeneric              ServiceError = "kimlik doğrulaması sırasında bir hata oluştu"
	ErrProfileGeneric           ServiceError = "profil bilgileri alınırken hata"
	ErrUpdatePasswordGeneric    ServiceError = "şifre güncellenirken bir hata oluştu"
	ErrHashingFailed            ServiceError = "yeni şifre oluşturulurken hata"
	ErrDatabaseUpdateFailed     ServiceError = "veritabanı güncellemesi başarısız oldu"
)

type IAuthService interface {
	Authenticate(email, password string) (*models.User, error)
	GetUserProfile(id uint) (*models.User, error)
	UpdatePassword(ctx context.Context, userID uint, currentPass, newPassword string) error
	CreateUser(ctx context.Context, user *models.User) error
	SendPasswordResetLink(email string) error
	ResetPassword(token, newPassword string) error
	VerifyEmail(token string) error
	ResendVerificationLink(email string) error
	FindOrCreateUser(user models.User) (*models.User, error)
}

type AuthService struct {
	repo repositories.IAuthRepository
}

func NewAuthService() IAuthService {
	return &AuthService{repo: repositories.NewAuthRepository()}
}

func (s *AuthService) logAuthSuccess(email string, userID uint) {
	logconfig.Log.Info("Kimlik doğrulama başarılı",
		zap.String("email", email),
		zap.Uint("user_id", userID),
	)
}

func (s *AuthService) logDBError(action string, err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	logconfig.Log.Error(action+" hatası (DB)", fields...)
}

func (s *AuthService) logWarn(action string, fields ...zap.Field) {
	logconfig.Log.Warn(action+" başarısız", fields...)
}

func (s *AuthService) getUserByEmail(email string) (*models.User, error) {
	user, err := s.repo.FindUserByEmail(email)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logWarn("Kullanıcı bulunamadı", zap.String("email", email))
			return nil, ErrUserNotFound
		}
		s.logDBError("Kullanıcı sorgulama", err, zap.String("email", email))
		return nil, ErrAuthGeneric
	}
	return user, nil
}

func (s *AuthService) getUserByID(id uint) (*models.User, error) {
	user, err := s.repo.FindUserByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logWarn("Kullanıcı bulunamadı", zap.Uint("user_id", id))
			return nil, ErrUserNotFound
		}
		s.logDBError("Kullanıcı sorgulama", err, zap.Uint("user_id", id))
		return nil, ErrProfileGeneric
	}
	return user, nil
}

func (s *AuthService) comparePasswords(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

func (s *AuthService) hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func (s *AuthService) Authenticate(email, password string) (*models.User, error) {
	user, err := s.getUserByEmail(email)
	if err != nil {
		return nil, err
	}

	if !user.Status {
		s.logWarn("Kullanıcı aktif değil",
			zap.String("email", email),
			zap.Uint("user_id", user.ID),
		)
		return nil, ErrUserInactive
	}

	if err := s.comparePasswords(user.Password, password); err != nil {
		s.logWarn("Geçersiz parola",
			zap.String("email", email),
			zap.Uint("user_id", user.ID),
		)
		return nil, ErrInvalidCredentials
	}

	s.logAuthSuccess(email, user.ID)
	return user, nil
}

func (s *AuthService) GetUserProfile(id uint) (*models.User, error) {
	return s.getUserByID(id)
}

func (s *AuthService) UpdatePassword(ctx context.Context, userID uint, currentPass, newPassword string) error {
	user, err := s.getUserByID(userID)
	if err != nil {
		return err
	}

	if err := s.comparePasswords(user.Password, currentPass); err != nil {
		s.logWarn("Mevcut parola hatalı", zap.Uint("user_id", userID))
		return ErrCurrentPasswordIncorrect
	}

	if len(newPassword) < 6 {
		s.logWarn("Yeni parola çok kısa", zap.Uint("user_id", userID))
		return ErrPasswordTooShort
	}

	if currentPass == newPassword {
		s.logWarn("Yeni parola eskiyle aynı", zap.Uint("user_id", userID))
		return ErrPasswordSameAsOld
	}

	hashedPassword, err := s.hashPassword(newPassword)
	if err != nil {
		s.logDBError("Parola hashleme", err, zap.Uint("user_id", userID))
		return ErrHashingFailed
	}

	user.Password = hashedPassword
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		s.logDBError("Kullanıcı güncelleme", err, zap.Uint("user_id", userID))
		return ErrDatabaseUpdateFailed
	}

	logconfig.Log.Info("Parola başarıyla güncellendi", zap.Uint("user_id", userID))
	return nil
}

func (s *AuthService) CreateUser(ctx context.Context, user *models.User) error {
	if user.Password == "" {
		return errors.New("şifre alanı boş olamaz")
	}
	if err := user.SetPassword(user.Password); err != nil {
		logconfig.Log.Error("Şifre oluşturulamadı", zap.Error(err))
		return errors.New("şifre oluşturulurken hata oluştu")
	}
	return s.repo.CreateUser(ctx, user)
}

func (s *AuthService) SendPasswordResetLink(email string) error {
	user, err := s.repo.FindUserByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return ErrAuthGeneric
	}

	// Generate reset token
	resetToken := generateToken() // Replace with actual token generation logic
	user.ResetToken = resetToken

	if err := s.repo.UpdateUser(context.Background(), user); err != nil {
		return ErrDatabaseUpdateFailed
	}

	// Send reset email
	mailService := NewMailService()
	resetLink := os.Getenv("APP_BASE_URL") + "/auth/reset-password?token=" + resetToken
	emailBody := "Şifrenizi sıfırlamak için aşağıdaki bağlantıya tıklayın: " + resetLink

	if err := mailService.SendMail(user.Email, "Şifre Sıfırlama", emailBody); err != nil {
		return fmt.Errorf("şifre sıfırlama e-postası gönderilemedi: %w", err)
	}

	return nil
}

func (s *AuthService) ResetPassword(token, newPassword string) error {
	user, err := s.repo.FindUserByResetToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return ErrAuthGeneric
	}

	if err := user.SetPassword(newPassword); err != nil {
		return ErrHashingFailed
	}

	user.ResetToken = "" // Clear the token

	if err := s.repo.UpdateUser(context.Background(), user); err != nil {
		return ErrDatabaseUpdateFailed
	}

	return nil
}

func (s *AuthService) VerifyEmail(token string) error {
	user, err := s.repo.FindUserByVerificationToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return ErrAuthGeneric
	}

	user.EmailVerified = true
	user.VerificationToken = "" // Clear the token

	if err := s.repo.UpdateUser(context.Background(), user); err != nil {
		return ErrDatabaseUpdateFailed
	}

	return nil
}

func (s *AuthService) ResendVerificationLink(email string) error {
	user, err := s.repo.FindUserByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return ErrAuthGeneric
	}
	if user.EmailVerified {
		return nil
	}
	verificationToken := generateToken()
	user.VerificationToken = verificationToken
	if err := s.repo.UpdateUser(context.Background(), user); err != nil {
		return ErrDatabaseUpdateFailed
	}
	mailService := NewMailService()
	baseURL := os.Getenv("APP_BASE_URL")
	verificationLink := baseURL + "/auth/verify-email?token=" + verificationToken
	emailBody := "Lütfen email adresinizi doğrulamak için aşağıdaki bağlantıya tıklayın: " + verificationLink
	return mailService.SendMail(user.Email, "Email Doğrulama", emailBody)
}

func generateToken() string {
	tokenBytes := make([]byte, 16) // 16 byte = 128 bit
	if _, err := rand.Read(tokenBytes); err != nil {
		logconfig.Log.Error("Token oluşturulamadı", zap.Error(err))
		return ""
	}
	return hex.EncodeToString(tokenBytes)
}

func (s *AuthService) FindOrCreateUser(user models.User) (*models.User, error) {
	existingUser, err := s.repo.FindByProviderAndID(user.Provider, user.ProviderID)
	if err == nil {
		return existingUser, nil
	}

	if err := s.repo.CreateUser(context.Background(), &user); err != nil {
		return nil, err
	}

	return &user, nil
}

var _ IAuthService = (*AuthService)(nil)
