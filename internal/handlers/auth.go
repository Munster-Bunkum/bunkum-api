package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/munster-bunkum/bunkum-api/internal/auth"
	"github.com/munster-bunkum/bunkum-api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Register(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Email == "" || req.Password == "" {
			jsonError(w, "username, email, and password are required", http.StatusUnprocessableEntity)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		user, err := db.CreateUser(r.Context(), pool, req.Username, req.Email, string(hash))
		if err != nil {
			// Postgres unique violation code is 23505
			if isUniqueViolation(err) {
				jsonError(w, "username or email already taken", http.StatusUnprocessableEntity)
				return
			}
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		token, err := auth.Encode(user.ID, user.Username)
		if err != nil {
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		jsonResponse(w, http.StatusCreated, map[string]any{
			"token": token,
			"user":  user,
		})
	}
}

func Login(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		user, err := db.FindUserByUsername(r.Context(), pool, req.Username)
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "invalid username or password", http.StatusUnauthorized)
			return
		}
		if err != nil {
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordDigest), []byte(req.Password)); err != nil {
			jsonError(w, "invalid username or password", http.StatusUnauthorized)
			return
		}

		token, err := auth.Encode(user.ID, user.Username)
		if err != nil {
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"token": token,
			"user":  user,
		})
	}
}
