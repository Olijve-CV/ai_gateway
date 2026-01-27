package services

import (
	"errors"

	"ai_gateway/internal/config"
	"ai_gateway/internal/database"
	"ai_gateway/internal/utils"

	"gorm.io/gorm"
)

// AuthService handles authentication operations
type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
}

// NewAuthService creates a new AuthService
func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Register registers a new user
func (s *AuthService) Register(req *RegisterRequest) (*database.User, error) {
	// Check if email already exists
	var existingUser database.User
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("email already registered")
	}

	// Check if username already exists
	if err := s.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, errors.New("username already taken")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Create user
	user := &database.User{
		Username:       req.Username,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		IsActive:       true,
		IsAdmin:        false,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// Authenticate authenticates a user with email and password
func (s *AuthService) Authenticate(email, password string) (*database.User, error) {
	var user database.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, errors.New("invalid email or password")
	}

	if !utils.VerifyPassword(password, user.HashedPassword) {
		return nil, errors.New("invalid email or password")
	}

	if !user.IsActive {
		return nil, errors.New("user is inactive")
	}

	return &user, nil
}

// CreateToken creates a JWT token for a user
func (s *AuthService) CreateToken(user *database.User) (string, error) {
	return utils.CreateAccessToken(user.ID, s.cfg.JWTSecret, s.cfg.JWTExpiration)
}

// GetUserByID gets a user by ID
func (s *AuthService) GetUserByID(userID uint) (*database.User, error) {
	var user database.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
