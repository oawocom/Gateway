package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type agentRow struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	Enabled      bool      `json:"enabled"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ---------------- superadmin: AI agents ----------------

func (s *Server) adminListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Master.Query(r.Context(),
		"SELECT id, name, description, system_prompt, enabled, updated_at FROM ai_agents ORDER BY created_at DESC")
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	list := []agentRow{}
	for rows.Next() {
		var a agentRow
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.SystemPrompt, &a.Enabled, &a.UpdatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, a)
	}
	writeJSON(w, 200, map[string]any{"agents": list})
}

func (s *Server) adminCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		errJSON(w, 400, "ad tələb olunur")
		return
	}
	var id string
	err := s.Master.QueryRow(r.Context(),
		"INSERT INTO ai_agents (name, description, system_prompt) VALUES ($1,$2,$3) RETURNING id",
		req.Name, req.Description, req.SystemPrompt).Scan(&id)
	if err != nil {
		errJSON(w, 500, "could not create agent")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Server) adminUpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		SystemPrompt *string `json:"system_prompt"`
		Enabled      *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	ctx := r.Context()
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		s.Master.Exec(ctx, "UPDATE ai_agents SET name=$1, updated_at=now() WHERE id=$2", strings.TrimSpace(*req.Name), id)
	}
	if req.Description != nil {
		s.Master.Exec(ctx, "UPDATE ai_agents SET description=$1, updated_at=now() WHERE id=$2", *req.Description, id)
	}
	if req.SystemPrompt != nil {
		s.Master.Exec(ctx, "UPDATE ai_agents SET system_prompt=$1, updated_at=now() WHERE id=$2", *req.SystemPrompt, id)
	}
	if req.Enabled != nil {
		s.Master.Exec(ctx, "UPDATE ai_agents SET enabled=$1, updated_at=now() WHERE id=$2", *req.Enabled, id)
	}
	writeJSON(w, 200, map[string]string{"id": id})
}

func (s *Server) adminDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag, err := s.Master.Exec(r.Context(), "DELETE FROM ai_agents WHERE id=$1", id)
	if err != nil || tag.RowsAffected() == 0 {
		errJSON(w, 404, "agent not found")
		return
	}
	writeJSON(w, 200, map[string]string{"id": id, "deleted": "true"})
}

// listAgents: enabled agents for tenant users (prompt hidden).
func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Master.Query(r.Context(),
		"SELECT id, name, description, enabled, updated_at FROM ai_agents WHERE enabled ORDER BY name")
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	list := []agentRow{}
	for rows.Next() {
		var a agentRow
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Enabled, &a.UpdatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, a)
	}
	writeJSON(w, 200, map[string]any{"agents": list})
}

// ---------------- superadmin: platform admins ----------------

func (s *Server) adminListAdmins(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Master.Query(r.Context(),
		"SELECT id, email, created_at FROM platform_admins ORDER BY created_at")
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	type row struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}
	list := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.Email, &x.CreatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, x)
	}
	writeJSON(w, 200, map[string]any{"admins": list})
}

func (s *Server) adminCreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if !strings.Contains(req.Email, "@") || len(req.Password) < 8 {
		errJSON(w, 400, "valid email and password (min 8) required")
		return
	}
	ctx := r.Context()
	var exists bool
	if err := s.Master.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=$1) OR EXISTS(SELECT 1 FROM platform_admins WHERE lower(email)=$1)",
		req.Email).Scan(&exists); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if exists {
		errJSON(w, 409, "email already registered")
		return
	}
	hash, err := hashPw(req.Password)
	if err != nil {
		errJSON(w, 500, "internal error")
		return
	}
	var id string
	if err := s.Master.QueryRow(ctx,
		"INSERT INTO platform_admins (email, password_hash) VALUES ($1,$2) RETURNING id",
		req.Email, hash).Scan(&id); err != nil {
		errJSON(w, 500, "could not create admin")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "email": req.Email})
}

func (s *Server) adminUpdateAdmin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	ctx := r.Context()

	var exists bool
	if err := s.Master.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform_admins WHERE id=$1)", id).Scan(&exists); err != nil || !exists {
		errJSON(w, 404, "admin not found")
		return
	}

	if req.Email != "" {
		if !strings.Contains(req.Email, "@") {
			errJSON(w, 400, "invalid email")
			return
		}
		var taken bool
		if err := s.Master.QueryRow(ctx,
			"SELECT (EXISTS(SELECT 1 FROM platform_admins WHERE lower(email)=$1 AND id<>$2) OR EXISTS(SELECT 1 FROM users WHERE lower(email)=$1))",
			req.Email, id).Scan(&taken); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		if taken {
			errJSON(w, 409, "email already registered")
			return
		}
		s.Master.Exec(ctx, "UPDATE platform_admins SET email=$1 WHERE id=$2", req.Email, id)
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			errJSON(w, 400, "password min 8 chars")
			return
		}
		hash, err := hashPw(req.Password)
		if err != nil {
			errJSON(w, 500, "internal error")
			return
		}
		s.Master.Exec(ctx, "UPDATE platform_admins SET password_hash=$1 WHERE id=$2", hash, id)
		s.Master.Exec(ctx, "UPDATE refresh_tokens SET revoked=true WHERE user_id=$1", id)
	}
	writeJSON(w, 200, map[string]string{"id": id, "updated": "true"})
}

func (s *Server) adminDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	claims := currentClaims(r)
	if claims != nil && claims.Subject == id {
		errJSON(w, 400, "özünüzü silə bilməzsiniz")
		return
	}
	var count int
	if err := s.Master.QueryRow(r.Context(), "SELECT count(*) FROM platform_admins").Scan(&count); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if count <= 1 {
		errJSON(w, 400, "son admin silinə bilməz")
		return
	}
	tag, err := s.Master.Exec(r.Context(), "DELETE FROM platform_admins WHERE id=$1", id)
	if err != nil || tag.RowsAffected() == 0 {
		errJSON(w, 404, "admin not found")
		return
	}
	s.Master.Exec(r.Context(), "UPDATE refresh_tokens SET revoked=true WHERE user_id=$1", id)
	writeJSON(w, 200, map[string]string{"id": id, "deleted": "true"})
}

// adminDeleteTenant permanently removes a tenant and drops its database.
// Requires body {"confirm": "<tenant slug>"} as an extra safety check.
func (s *Server) adminDeleteTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	ctx := r.Context()

	var slug, dbName string
	if err := s.Master.QueryRow(ctx, "SELECT slug, db_name FROM tenants WHERE id=$1", id).Scan(&slug, &dbName); err != nil {
		errJSON(w, 404, "tenant not found")
		return
	}
	if strings.TrimSpace(req.Confirm) != slug {
		errJSON(w, 400, "təsdiq mətni slug ilə uyğun gəlmir")
		return
	}

	// revoke sessions of tenant users, then delete tenant (users cascade)
	s.Master.Exec(ctx, "UPDATE refresh_tokens SET revoked=true WHERE user_id IN (SELECT id FROM users WHERE tenant_id=$1)", id)
	if _, err := s.Master.Exec(ctx, "DELETE FROM tenants WHERE id=$1", id); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if err := s.Tenants.Drop(ctx, dbName); err != nil {
		// tenant row already gone; report drop failure explicitly
		errJSON(w, 500, "tenant silindi, lakin baza silinmədi: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"id": id, "deleted": "true", "db_dropped": dbName})
}
