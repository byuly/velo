package domain

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxDisplayNameLength = 40

type User struct {
	ID          uuid.UUID `json:"id"`
	AppleSub    string    `json:"apple_sub"`
	DisplayName *string   `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url"`
	APNsToken   *string   `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidateUpdate checks mutable user fields.
func (u *User) ValidateUpdate() error {
	if u.DisplayName != nil && utf8.RuneCountInString(*u.DisplayName) > MaxDisplayNameLength {
		return ValidationErrorf("display_name must be at most %d characters", MaxDisplayNameLength)
	}
	return nil
}
