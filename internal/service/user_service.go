package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
)

func (s *Service) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	logger := log.Ctx(ctx).With().Str("func", "Register").Logger()
	logger.Debug().Str("email", req.Email).Msg("registering user")

	if req.Email == "" || req.Password == "" || req.Name == "" {
		logger.Warn().Msg("missing required registration fields")
		return nil, errors.New("name, email, and password are required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().Err(err).Msg("failed to hash password")
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		logger.Error().Err(err).Str("email", req.Email).Msg("failed to create user in repo")
		return nil, err
	}

	token, refreshToken, err := s.generateTokens(user)
	if err != nil {
		logger.Error().Err(err).Int64("user_id", user.ID).Msg("failed to generate tokens")
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	logger.Info().Int64("user_id", user.ID).Str("email", user.Email).Msg("user registered successfully")
	return &models.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	logger := log.Ctx(ctx).With().Str("func", "Login").Logger()
	logger.Debug().Str("email", req.Email).Msg("logging in user")

	if req.Email == "" || req.Password == "" {
		logger.Warn().Msg("missing email or password")
		return nil, errors.New("email and password are required")
	}

	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			logger.Warn().Str("email", req.Email).Msg("user not found during login")
			return nil, ErrInvalidCredentials
		}
		logger.Error().Err(err).Str("email", req.Email).Msg("failed to fetch user by email")
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logger.Warn().Str("email", req.Email).Msg("invalid password attempt")
		return nil, ErrInvalidCredentials
	}

	token, refreshToken, err := s.generateTokens(user)
	if err != nil {
		logger.Error().Err(err).Int64("user_id", user.ID).Msg("failed to generate tokens during login")
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	logger.Info().Int64("user_id", user.ID).Str("email", user.Email).Msg("user logged in successfully")
	return &models.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Service) ValidateToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func (s *Service) generateTokens(user *models.User) (string, string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", err
	}

	// Refresh Token (valid for 7 days)
	refreshClaims := &JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshTokenObj.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", err
	}

	return tokenString, refreshTokenString, nil
}
