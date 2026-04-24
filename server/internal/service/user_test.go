package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
	"github.com/byuly/velo/server/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// --- UserService mocks ---

type userSvcUserRepo struct {
	getByID      domain.User
	getByIDErr   error
	getByIDCalls int

	updateDisplayNameName   string
	updateDisplayNameCalled bool
	updateDisplayNameErr    error

	deleteCalled bool
	deleteErr    error
}

func (m *userSvcUserRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.User, error) {
	m.getByIDCalls++
	return m.getByID, m.getByIDErr
}
func (m *userSvcUserRepo) GetByAppleSub(_ context.Context, _ string) (domain.User, error) {
	panic("not implemented")
}
func (m *userSvcUserRepo) UpsertByAppleSub(_ context.Context, _ string) (domain.User, error) {
	panic("not implemented")
}
func (m *userSvcUserRepo) Delete(_ context.Context, _ uuid.UUID) error {
	m.deleteCalled = true
	return m.deleteErr
}
func (m *userSvcUserRepo) UpdateDisplayName(_ context.Context, _ uuid.UUID, name string) error {
	m.updateDisplayNameCalled = true
	m.updateDisplayNameName = name
	return m.updateDisplayNameErr
}
func (m *userSvcUserRepo) UpdateAvatarURL(_ context.Context, _ uuid.UUID, _ string) error {
	panic("not implemented")
}
func (m *userSvcUserRepo) UpdateAPNsToken(_ context.Context, _ uuid.UUID, _ string) error {
	panic("not implemented")
}

var _ repository.UserRepository = (*userSvcUserRepo)(nil)

func sptr(s string) *string { return &s }

// Reuses mockTokenRepo from auth_test.go (same package service_test).

// --- GetMe ---

func TestGetMe_Success(t *testing.T) {
	userID := uuid.New()
	users := &userSvcUserRepo{getByID: domain.User{ID: userID, DisplayName: sptr("alice")}}
	svc := service.NewUserService(users, &mockTokenRepo{})

	got, err := svc.GetMe(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, userID, got.ID)
}

func TestGetMe_NotFound(t *testing.T) {
	users := &userSvcUserRepo{getByIDErr: domain.ErrNotFound}
	svc := service.NewUserService(users, &mockTokenRepo{})

	_, err := svc.GetMe(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- UpdateMe ---

func TestUpdateMe_Success(t *testing.T) {
	userID := uuid.New()
	newName := "bob"
	users := &userSvcUserRepo{getByID: domain.User{ID: userID, DisplayName: &newName}}
	svc := service.NewUserService(users, &mockTokenRepo{})

	got, err := svc.UpdateMe(context.Background(), userID, domain.User{DisplayName: &newName})
	require.NoError(t, err)
	require.True(t, users.updateDisplayNameCalled)
	require.Equal(t, "bob", users.updateDisplayNameName)
	require.NotNil(t, got.DisplayName)
	require.Equal(t, "bob", *got.DisplayName)
}

func TestUpdateMe_TooLong(t *testing.T) {
	users := &userSvcUserRepo{}
	svc := service.NewUserService(users, &mockTokenRepo{})

	tooLong := strings.Repeat("a", domain.MaxDisplayNameLength+1)
	_, err := svc.UpdateMe(context.Background(), uuid.New(), domain.User{DisplayName: &tooLong})
	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 400, appErr.Status)
	require.False(t, users.updateDisplayNameCalled)
}

func TestUpdateMe_NilDisplayName_NoOp(t *testing.T) {
	userID := uuid.New()
	users := &userSvcUserRepo{getByID: domain.User{ID: userID}}
	svc := service.NewUserService(users, &mockTokenRepo{})

	_, err := svc.UpdateMe(context.Background(), userID, domain.User{})
	require.NoError(t, err)
	require.False(t, users.updateDisplayNameCalled)
	require.Equal(t, 1, users.getByIDCalls)
}

func TestUpdateMe_RepoError(t *testing.T) {
	users := &userSvcUserRepo{updateDisplayNameErr: errors.New("db down")}
	svc := service.NewUserService(users, &mockTokenRepo{})

	name := "bob"
	_, err := svc.UpdateMe(context.Background(), uuid.New(), domain.User{DisplayName: &name})
	require.Error(t, err)
}

// --- DeleteMe ---

func TestDeleteMe_Success(t *testing.T) {
	users := &userSvcUserRepo{}
	tokens := &mockTokenRepo{}
	svc := service.NewUserService(users, tokens)

	err := svc.DeleteMe(context.Background(), uuid.New())
	require.NoError(t, err)
	require.True(t, tokens.deleteByUserIDCalled)
	require.True(t, users.deleteCalled)
}

func TestDeleteMe_TokenDeleteFails(t *testing.T) {
	users := &userSvcUserRepo{}
	tokens := &mockTokenRepo{deleteByUserIDErr: errors.New("db down")}
	svc := service.NewUserService(users, tokens)

	err := svc.DeleteMe(context.Background(), uuid.New())
	require.Error(t, err)
	require.False(t, users.deleteCalled)
}

func TestDeleteMe_UserDeleteFails(t *testing.T) {
	users := &userSvcUserRepo{deleteErr: errors.New("db down")}
	tokens := &mockTokenRepo{}
	svc := service.NewUserService(users, tokens)

	err := svc.DeleteMe(context.Background(), uuid.New())
	require.Error(t, err)
	require.True(t, tokens.deleteByUserIDCalled)
}
