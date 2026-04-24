package service

import (
	"context"
	"fmt"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
	"github.com/google/uuid"
)

// UserService handles profile reads and mutations for the authenticated user.
type UserService interface {
	GetMe(ctx context.Context, userID uuid.UUID) (domain.User, error)
	UpdateMe(ctx context.Context, userID uuid.UUID, update domain.User) (domain.User, error)
	DeleteMe(ctx context.Context, userID uuid.UUID) error
}

var _ UserService = (*userService)(nil)

type userService struct {
	users  repository.UserRepository
	tokens repository.TokenRepository
}

func NewUserService(users repository.UserRepository, tokens repository.TokenRepository) UserService {
	return &userService{users: users, tokens: tokens}
}

func (s *userService) GetMe(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *userService) UpdateMe(ctx context.Context, userID uuid.UUID, update domain.User) (domain.User, error) {
	if err := update.ValidateUpdate(); err != nil {
		return domain.User{}, err
	}

	if update.DisplayName != nil {
		if err := s.users.UpdateDisplayName(ctx, userID, *update.DisplayName); err != nil {
			return domain.User{}, fmt.Errorf("update display name: %w", err)
		}
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *userService) DeleteMe(ctx context.Context, userID uuid.UUID) error {
	if err := s.tokens.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete refresh tokens: %w", err)
	}
	if err := s.users.Delete(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
