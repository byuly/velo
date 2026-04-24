package handler

import (
	"net/http"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/service"
)

type UserHandler struct {
	svc service.UserService
}

func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		Error(w, domain.ErrUnauthorized)
		return
	}

	user, err := h.svc.GetMe(r.Context(), userID)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		Error(w, domain.ErrUnauthorized)
		return
	}

	var body struct {
		DisplayName *string `json:"display_name"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, err)
		return
	}

	user, err := h.svc.UpdateMe(r.Context(), userID, domain.User{DisplayName: body.DisplayName})
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		Error(w, domain.ErrUnauthorized)
		return
	}

	if err := h.svc.DeleteMe(r.Context(), userID); err != nil {
		Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
