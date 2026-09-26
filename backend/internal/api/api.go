package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gateway/internal/auth"
	"gateway/internal/crypto"
	"gateway/internal/syncer"
	"gateway/internal/tenantdb"
)

type Server struct {
	Master  *pgxpool.Pool
	JWT     *auth.JWT
	Tenants *tenantdb.Manager
	Box     *crypto.Box
	Sync    *syncer.Syncer
}

type ctxKey int

const claimsKey ctxKey = 1

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("POST /api/v1/auth/register", s.register)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.refresh)
	mux.Handle("GET /api/v1/me", s.requireAuth(http.HandlerFunc(s.me)))

	// superadmin
	mux.Handle("GET /api/v1/admin/tenants", s.requireSuperadmin(http.HandlerFunc(s.adminListTenants)))
	mux.Handle("PATCH /api/v1/admin/tenants/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminUpdateTenant)))
	mux.Handle("POST /api/v1/admin/tenants", s.requireSuperadmin(http.HandlerFunc(s.adminCreateTenant)))
	mux.Handle("DELETE /api/v1/admin/tenants/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminDeleteTenant)))
	mux.Handle("GET /api/v1/admin/tenants/{id}/users", s.requireSuperadmin(http.HandlerFunc(s.adminListTenantUsers)))
	mux.Handle("PATCH /api/v1/admin/users/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminUpdateUser)))

	// superadmin: AI agents + admins
	mux.Handle("GET /api/v1/admin/agents", s.requireSuperadmin(http.HandlerFunc(s.adminListAgents)))
	mux.Handle("POST /api/v1/admin/agents", s.requireSuperadmin(http.HandlerFunc(s.adminCreateAgent)))
	mux.Handle("PATCH /api/v1/admin/agents/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminUpdateAgent)))
	mux.Handle("DELETE /api/v1/admin/agents/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminDeleteAgent)))
	mux.Handle("GET /api/v1/admin/admins", s.requireSuperadmin(http.HandlerFunc(s.adminListAdmins)))
	mux.Handle("POST /api/v1/admin/admins", s.requireSuperadmin(http.HandlerFunc(s.adminCreateAdmin)))
	mux.Handle("PATCH /api/v1/admin/admins/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminUpdateAdmin)))
	mux.Handle("DELETE /api/v1/admin/admins/{id}", s.requireSuperadmin(http.HandlerFunc(s.adminDeleteAdmin)))
	mux.Handle("GET /api/v1/agents", s.requireTenant(http.HandlerFunc(s.listAgents)))

	// tenant users
	mux.Handle("GET /api/v1/users", s.requireRole(http.HandlerFunc(s.listUsers), "owner", "admin"))
	mux.Handle("POST /api/v1/users", s.requireRole(http.HandlerFunc(s.createUser), "owner", "admin"))
	mux.Handle("PATCH /api/v1/users/{id}", s.requireRole(http.HandlerFunc(s.updateUserRole), "owner", "admin"))
	mux.Handle("DELETE /api/v1/users/{id}", s.requireRole(http.HandlerFunc(s.deleteUser), "owner", "admin"))

	// connectors & connections
	mux.Handle("GET /api/v1/connectors", s.requireTenant(http.HandlerFunc(s.listConnectors)))
	mux.Handle("GET /api/v1/connections", s.requireTenant(http.HandlerFunc(s.listConnections)))
	mux.Handle("POST /api/v1/connections", s.requireRole(http.HandlerFunc(s.createConnection), "owner", "admin"))
	mux.Handle("POST /api/v1/connections/test", s.requireRole(http.HandlerFunc(s.testConnectionConfig), "owner", "admin"))
	mux.Handle("POST /api/v1/connections/{id}/test", s.requireRole(http.HandlerFunc(s.testConnection), "owner", "admin"))
	mux.Handle("GET /api/v1/connections/{id}/entities", s.requireTenant(http.HandlerFunc(s.listConnectionEntities)))
	mux.Handle("GET /api/v1/connections/{id}/c1meta", s.requireTenant(http.HandlerFunc(s.connectionC1Meta)))
	mux.Handle("POST /api/v1/connections/{id}/sync", s.requireRole(http.HandlerFunc(s.syncConnection), "owner", "admin"))
	mux.Handle("DELETE /api/v1/connections/{id}", s.requireRole(http.HandlerFunc(s.deleteConnection), "owner", "admin"))

	// data
	mux.Handle("GET /api/v1/data/summary", s.requireTenant(http.HandlerFunc(s.dataSummary)))
	mux.Handle("GET /api/v1/data/records", s.requireTenant(http.HandlerFunc(s.dataRecords)))
	mux.Handle("GET /api/v1/data/synclog", s.requireTenant(http.HandlerFunc(s.syncLog)))
	mux.Handle("GET /api/v1/reports/summary", s.requireTenant(http.HandlerFunc(s.reportsSummary)))
	mux.Handle("GET /api/v1/reports/1c", s.requireTenant(http.HandlerFunc(s.reports1C)))

	// automations
	mux.Handle("GET /api/v1/automations", s.requireTenant(http.HandlerFunc(s.listAutomations)))
	mux.Handle("POST /api/v1/automations", s.requireRole(http.HandlerFunc(s.createAutomation), "owner", "admin"))
	mux.Handle("PATCH /api/v1/automations/{id}", s.requireRole(http.HandlerFunc(s.updateAutomation), "owner", "admin"))
	mux.Handle("DELETE /api/v1/automations/{id}", s.requireRole(http.HandlerFunc(s.deleteAutomation), "owner", "admin"))

	// stats
	mux.Handle("GET /api/v1/stats", s.requireTenant(http.HandlerFunc(s.stats)))

	return mux
}


// hashPw wraps auth.HashPassword for sibling files.
func hashPw(pw string) (string, error) { return auth.HashPassword(pw) }

// currentClaims returns JWT claims stored by requireAuth.
func currentClaims(r *http.Request) *auth.Claims {
	if v := r.Context().Value(claimsKey); v != nil {
		return v.(*auth.Claims)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	pendingSep := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
		} else {
			pendingSep = true
		}
	}
	return b.String()
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	dbOK := s.Master.Ping(ctx) == nil
	code := http.StatusOK
	if !dbOK {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"status": "ok", "db": dbOK})
}

type registerReq struct {
	Company  string `json:"company"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Company = strings.TrimSpace(req.Company)
	if req.Company == "" || !strings.Contains(req.Email, "@") || len(req.Password) < 8 {
		errJSON(w, 400, "company, valid email and password (min 8 chars) required")
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

	slug := slugify(req.Company)
	if slug == "" {
		errJSON(w, 400, "invalid company name")
		return
	}
	if err := s.Master.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tenants WHERE slug=$1)", slug).Scan(&exists); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if exists {
		slug = slug + "_" + randHex(3)
	}
	dbName := "tenant_" + slug

	var tenantID string
	err := s.Master.QueryRow(ctx,
		"INSERT INTO tenants (slug, name, db_name, status) VALUES ($1,$2,$3,'provisioning') RETURNING id",
		slug, req.Company, dbName).Scan(&tenantID)
	if err != nil {
		log.Printf("insert tenant: %v", err)
		errJSON(w, 500, "could not create tenant")
		return
	}

	if err := s.Tenants.Provision(ctx, dbName); err != nil {
		log.Printf("provision %s: %v", dbName, err)
		s.Master.Exec(ctx, "UPDATE tenants SET status='failed' WHERE id=$1", tenantID)
		errJSON(w, 500, "tenant provisioning failed")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		errJSON(w, 500, "internal error")
		return
	}

	var userID string
	err = s.Master.QueryRow(ctx,
		"INSERT INTO users (tenant_id, email, password_hash, role) VALUES ($1,$2,$3,'owner') RETURNING id",
		tenantID, req.Email, hash).Scan(&userID)
	if err != nil {
		log.Printf("insert user: %v", err)
		errJSON(w, 500, "could not create user")
		return
	}

	s.Master.Exec(ctx, "UPDATE tenants SET status='active' WHERE id=$1", tenantID)

	s.issueTokens(w, r, userID, tenantID, dbName, "owner", map[string]any{
		"user":   map[string]string{"id": userID, "email": req.Email, "role": "owner"},
		"tenant": map[string]string{"id": tenantID, "name": req.Company, "slug": slug},
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
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

	// platform admin first
	var adminID, adminHash string
	err := s.Master.QueryRow(ctx, "SELECT id, password_hash FROM platform_admins WHERE lower(email)=$1", req.Email).
		Scan(&adminID, &adminHash)
	if err == nil {
		if !auth.CheckPassword(adminHash, req.Password) {
			errJSON(w, 401, "invalid credentials")
			return
		}
		s.issueTokens(w, r, adminID, "", "", "superadmin", map[string]any{
			"user": map[string]string{"id": adminID, "email": req.Email, "role": "superadmin"},
		})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		errJSON(w, 500, "db error")
		return
	}

	var userID, pwHash, role, tenantID, tName, tSlug, tDB, tStatus string
	err = s.Master.QueryRow(ctx, `SELECT u.id, u.password_hash, u.role, t.id, t.name, t.slug, t.db_name, t.status
		FROM users u JOIN tenants t ON t.id = u.tenant_id
		WHERE lower(u.email) = $1`, req.Email).
		Scan(&userID, &pwHash, &role, &tenantID, &tName, &tSlug, &tDB, &tStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			errJSON(w, 401, "invalid credentials")
			return
		}
		errJSON(w, 500, "db error")
		return
	}
	if !auth.CheckPassword(pwHash, req.Password) {
		errJSON(w, 401, "invalid credentials")
		return
	}
	if tStatus != "active" {
		errJSON(w, 403, "tenant is not active")
		return
	}
	s.issueTokens(w, r, userID, tenantID, tDB, role, map[string]any{
		"user":   map[string]string{"id": userID, "email": req.Email, "role": role},
		"tenant": map[string]string{"id": tenantID, "name": tName, "slug": tSlug},
	})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		errJSON(w, 400, "refresh_token required")
		return
	}
	ctx := r.Context()
	hash := auth.HashRefresh(req.RefreshToken)

	var tokenID, subjectID string
	err := s.Master.QueryRow(ctx,
		"SELECT id, user_id FROM refresh_tokens WHERE token_hash=$1 AND NOT revoked AND expires_at > now()",
		hash).Scan(&tokenID, &subjectID)
	if err != nil {
		errJSON(w, 401, "invalid refresh token")
		return
	}
	s.Master.Exec(ctx, "UPDATE refresh_tokens SET revoked=true WHERE id=$1", tokenID)

	var adminEmail string
	err = s.Master.QueryRow(ctx, "SELECT email FROM platform_admins WHERE id=$1", subjectID).Scan(&adminEmail)
	if err == nil {
		s.issueTokens(w, r, subjectID, "", "", "superadmin", map[string]any{
			"user": map[string]string{"id": subjectID, "email": adminEmail, "role": "superadmin"},
		})
		return
	}

	var email, role, tenantID, tName, tSlug, tDB, tStatus string
	err = s.Master.QueryRow(ctx, `SELECT u.email, u.role, t.id, t.name, t.slug, t.db_name, t.status
		FROM users u JOIN tenants t ON t.id = u.tenant_id WHERE u.id=$1`, subjectID).
		Scan(&email, &role, &tenantID, &tName, &tSlug, &tDB, &tStatus)
	if err != nil {
		errJSON(w, 401, "user not found")
		return
	}
	if tStatus != "active" {
		errJSON(w, 403, "tenant is not active")
		return
	}
	s.issueTokens(w, r, subjectID, tenantID, tDB, role, map[string]any{
		"user":   map[string]string{"id": subjectID, "email": email, "role": role},
		"tenant": map[string]string{"id": tenantID, "name": tName, "slug": tSlug},
	})
}

func (s *Server) issueTokens(w http.ResponseWriter, r *http.Request, userID, tenantID, dbName, role string, extra map[string]any) {
	access, err := s.JWT.Sign(userID, tenantID, dbName, role)
	if err != nil {
		errJSON(w, 500, "token error")
		return
	}
	raw, hash, err := auth.NewRefreshToken()
	if err != nil {
		errJSON(w, 500, "token error")
		return
	}
	if _, err := s.Master.Exec(r.Context(),
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1,$2,$3)",
		userID, hash, time.Now().Add(auth.RefreshTTL)); err != nil {
		errJSON(w, 500, "token error")
		return
	}
	resp := map[string]any{
		"access_token":  access,
		"refresh_token": raw,
		"expires_in":    int(auth.AccessTTL.Seconds()),
	}
	for k, v := range extra {
		resp[k] = v
	}
	writeJSON(w, 200, resp)
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			errJSON(w, 401, "missing token")
			return
		}
		claims, err := s.JWT.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			errJSON(w, 401, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

// requireTenant: any authenticated tenant user (not superadmin).
func (s *Server) requireTenant(next http.Handler) http.Handler {
	return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(claimsKey).(*auth.Claims)
		if claims.TenantID == "" || claims.TenantDB == "" {
			errJSON(w, 403, "tenant context required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// requireRole allows only listed tenant roles.
func (s *Server) requireRole(next http.Handler, roles ...string) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return s.requireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(claimsKey).(*auth.Claims)
		if !allowed[claims.Role] {
			errJSON(w, 403, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (s *Server) requireSuperadmin(next http.Handler) http.Handler {
	return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(claimsKey).(*auth.Claims)
		if claims.Role != "superadmin" {
			errJSON(w, 403, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// tenantPool returns the tenant DB pool for the current request.
func (s *Server) tenantPool(r *http.Request) (*pgxpool.Pool, *auth.Claims, error) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	pool, err := s.Tenants.Pool(r.Context(), claims.TenantDB)
	return pool, claims, err
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)

	if claims.Role == "superadmin" {
		var email string
		if err := s.Master.QueryRow(r.Context(), "SELECT email FROM platform_admins WHERE id=$1", claims.Subject).Scan(&email); err != nil {
			errJSON(w, 404, "user not found")
			return
		}
		writeJSON(w, 200, map[string]any{
			"user": map[string]string{"id": claims.Subject, "email": email, "role": "superadmin"},
		})
		return
	}

	var email, role, tName, tSlug string
	err := s.Master.QueryRow(r.Context(), `SELECT u.email, u.role, t.name, t.slug
		FROM users u JOIN tenants t ON t.id = u.tenant_id WHERE u.id=$1`, claims.Subject).
		Scan(&email, &role, &tName, &tSlug)
	if err != nil {
		errJSON(w, 404, "user not found")
		return
	}
	writeJSON(w, 200, map[string]any{
		"user":   map[string]string{"id": claims.Subject, "email": email, "role": role},
		"tenant": map[string]string{"id": claims.TenantID, "name": tName, "slug": tSlug},
	})
}

func (s *Server) adminListTenants(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Master.Query(r.Context(), `
		SELECT t.id, t.slug, t.name, t.db_name, t.status, t.created_at,
		       (SELECT count(*) FROM users u WHERE u.tenant_id = t.id) AS user_count
		FROM tenants t ORDER BY t.created_at DESC`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()

	type tenant struct {
		ID        string    `json:"id"`
		Slug      string    `json:"slug"`
		Name      string    `json:"name"`
		DBName    string    `json:"db_name"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
		UserCount int       `json:"user_count"`
	}
	list := []tenant{}
	for rows.Next() {
		var t tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.DBName, &t.Status, &t.CreatedAt, &t.UserCount); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, t)
	}
	writeJSON(w, 200, map[string]any{"tenants": list})
}

func (s *Server) adminUpdateTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	if req.Status != "active" && req.Status != "suspended" {
		errJSON(w, 400, "status must be 'active' or 'suspended'")
		return
	}
	tag, err := s.Master.Exec(r.Context(), "UPDATE tenants SET status=$1 WHERE id=$2", req.Status, id)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if tag.RowsAffected() == 0 {
		errJSON(w, 404, "tenant not found")
		return
	}
	writeJSON(w, 200, map[string]string{"id": id, "status": req.Status})
}

// ---------------- superadmin: tenant user management ----------------

func (s *Server) adminListTenantUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	rows, err := s.Master.Query(r.Context(),
		"SELECT id, email, role, created_at FROM users WHERE tenant_id=$1 ORDER BY created_at", tenantID)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	type u struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		CreatedAt time.Time `json:"created_at"`
	}
	list := []u{}
	for rows.Next() {
		var x u
		if err := rows.Scan(&x.ID, &x.Email, &x.Role, &x.CreatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, x)
	}
	writeJSON(w, 200, map[string]any{"users": list})
}

func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
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
	if err := s.Master.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)", id).Scan(&exists); err != nil || !exists {
		errJSON(w, 404, "user not found")
		return
	}

	if req.Email != "" {
		if !strings.Contains(req.Email, "@") {
			errJSON(w, 400, "invalid email")
			return
		}
		var taken bool
		if err := s.Master.QueryRow(ctx,
			"SELECT (EXISTS(SELECT 1 FROM users WHERE lower(email)=$1 AND id<>$2) OR EXISTS(SELECT 1 FROM platform_admins WHERE lower(email)=$1))",
			req.Email, id).Scan(&taken); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		if taken {
			errJSON(w, 409, "email already registered")
			return
		}
		if _, err := s.Master.Exec(ctx, "UPDATE users SET email=$1 WHERE id=$2", req.Email, id); err != nil {
			errJSON(w, 500, "db error")
			return
		}
	}

	if req.Password != "" {
		if len(req.Password) < 8 {
			errJSON(w, 400, "password min 8 chars")
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			errJSON(w, 500, "internal error")
			return
		}
		if _, err := s.Master.Exec(ctx, "UPDATE users SET password_hash=$1 WHERE id=$2", hash, id); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		// force re-login: revoke refresh tokens
		s.Master.Exec(ctx, "UPDATE refresh_tokens SET revoked=true WHERE user_id=$1", id)
	}

	writeJSON(w, 200, map[string]string{"id": id, "updated": "true"})
}

// adminCreateTenant lets the superadmin onboard a company manually.
func (s *Server) adminCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Company = strings.TrimSpace(req.Company)
	if req.Company == "" || !strings.Contains(req.Email, "@") || len(req.Password) < 8 {
		errJSON(w, 400, "company, valid email and password (min 8 chars) required")
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

	slug := slugify(req.Company)
	if slug == "" {
		errJSON(w, 400, "invalid company name")
		return
	}
	if err := s.Master.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tenants WHERE slug=$1)", slug).Scan(&exists); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if exists {
		slug = slug + "_" + randHex(3)
	}
	dbName := "tenant_" + slug

	var tenantID string
	err := s.Master.QueryRow(ctx,
		"INSERT INTO tenants (slug, name, db_name, status) VALUES ($1,$2,$3,'provisioning') RETURNING id",
		slug, req.Company, dbName).Scan(&tenantID)
	if err != nil {
		errJSON(w, 500, "could not create tenant")
		return
	}
	if err := s.Tenants.Provision(ctx, dbName); err != nil {
		log.Printf("provision %s: %v", dbName, err)
		s.Master.Exec(ctx, "UPDATE tenants SET status='failed' WHERE id=$1", tenantID)
		errJSON(w, 500, "tenant provisioning failed")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		errJSON(w, 500, "internal error")
		return
	}
	if _, err := s.Master.Exec(ctx,
		"INSERT INTO users (tenant_id, email, password_hash, role) VALUES ($1,$2,$3,'owner')",
		tenantID, req.Email, hash); err != nil {
		errJSON(w, 500, "could not create user")
		return
	}
	s.Master.Exec(ctx, "UPDATE tenants SET status='active' WHERE id=$1", tenantID)
	writeJSON(w, 201, map[string]string{"id": tenantID, "slug": slug, "name": req.Company})
}
