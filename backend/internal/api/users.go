package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gateway/internal/auth"
)

var validRoles = map[string]bool{"admin": true, "member": true}

type userRow struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	rows, err := s.Master.Query(r.Context(),
		"SELECT id, email, role, active, last_login_at, created_at FROM users WHERE tenant_id=$1 ORDER BY created_at", claims.TenantID)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	list := []userRow{}
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.Active, &u.LastLoginAt, &u.CreatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, u)
	}
	writeJSON(w, 200, map[string]any{"users": list})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if !strings.Contains(req.Email, "@") || len(req.Password) < 8 || !validRoles[req.Role] {
		errJSON(w, 400, "valid email, password (min 8) and role (admin|member) required")
		return
	}
	if claims.Role == "admin" && req.Role == "admin" {
		errJSON(w, 403, "only owner can create admins")
		return
	}
	ctx := r.Context()
	var exists bool
	if err := s.Master.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=$1) OR EXISTS(SELECT 1 FROM platform_admins WHERE lower(email)=$1)", req.Email).Scan(&exists); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if exists {
		errJSON(w, 409, "email already registered")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		errJSON(w, 500, "internal error")
		return
	}
	var id string
	err = s.Master.QueryRow(ctx,
		"INSERT INTO users (tenant_id, email, password_hash, role) VALUES ($1,$2,$3,$4) RETURNING id",
		claims.TenantID, req.Email, hash, req.Role).Scan(&id)
	if err != nil {
		errJSON(w, 500, "could not create user")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "email": req.Email, "role": req.Role})
}

func (s *Server) updateUserRole(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	id := r.PathValue("id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !validRoles[req.Role] {
		errJSON(w, 400, "role must be admin|member")
		return
	}
	if id == claims.Subject {
		errJSON(w, 400, "cannot change own role")
		return
	}
	var targetRole string
	if err := s.Master.QueryRow(r.Context(),
		"SELECT role FROM users WHERE id=$1 AND tenant_id=$2", id, claims.TenantID).Scan(&targetRole); err != nil {
		errJSON(w, 404, "user not found")
		return
	}
	if targetRole == "owner" {
		errJSON(w, 403, "cannot change owner role")
		return
	}
	if claims.Role == "admin" && (targetRole == "admin" || req.Role == "admin") {
		errJSON(w, 403, "only owner can manage admins")
		return
	}
	s.Master.Exec(r.Context(), "UPDATE users SET role=$1 WHERE id=$2", req.Role, id)
	writeJSON(w, 200, map[string]string{"id": id, "role": req.Role})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	id := r.PathValue("id")
	if id == claims.Subject {
		errJSON(w, 400, "cannot delete yourself")
		return
	}
	var targetRole string
	if err := s.Master.QueryRow(r.Context(),
		"SELECT role FROM users WHERE id=$1 AND tenant_id=$2", id, claims.TenantID).Scan(&targetRole); err != nil {
		errJSON(w, 404, "user not found")
		return
	}
	if targetRole == "owner" {
		errJSON(w, 403, "cannot delete owner")
		return
	}
	if claims.Role == "admin" && targetRole == "admin" {
		errJSON(w, 403, "only owner can delete admins")
		return
	}
	s.Master.Exec(r.Context(), "DELETE FROM users WHERE id=$1", id)
	writeJSON(w, 200, map[string]string{"id": id, "deleted": "true"})
}

// setUserActive — PATCH /users/{id}/active {active}: deactivate/reactivate.
// Deactivation blocks login; sessions expire with the token. Owner is
// untouchable; admins manage members only.
func (s *Server) setUserActive(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	id := r.PathValue("id")
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	if id == claims.Subject {
		errJSON(w, 400, "öz hesabınızı deaktiv edə bilməzsiniz")
		return
	}
	var targetRole string
	if err := s.Master.QueryRow(r.Context(),
		"SELECT role FROM users WHERE id=$1 AND tenant_id=$2", id, claims.TenantID).Scan(&targetRole); err != nil {
		errJSON(w, 404, "user not found")
		return
	}
	if targetRole == "owner" {
		errJSON(w, 403, "sahib hesabı deaktiv edilə bilməz")
		return
	}
	if claims.Role == "admin" && targetRole == "admin" {
		errJSON(w, 403, "adminləri yalnız sahib idarə edir")
		return
	}
	s.Master.Exec(r.Context(), "UPDATE users SET active=$1 WHERE id=$2", req.Active, id)
	writeJSON(w, 200, map[string]any{"id": id, "active": req.Active})
}
