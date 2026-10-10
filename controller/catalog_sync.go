package controller

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/internal/catalogtransport"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CatalogSyncManagementMiddleware protects target management methods, including
// preview/history reads. GET status has a dedicated read-only role-aware guard.
// This chain does not consume a security proof:
// apply/restore must precheck the stored intent before consuming that proof.
func CatalogSyncManagementMiddleware() []gin.HandlerFunc {
	runtime, err := service.CurrentCatalogSyncRuntime()
	return []gin.HandlerFunc{middleware.RootAuth(), catalogSyncManagementGuard(runtime, err)}
}

func catalogSyncManagementGuard(runtime *service.CatalogSyncRuntime, configurationError error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if configurationError != nil || runtime == nil || !runtime.Status().ManagementReady {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "CATALOG_SYNC_UNAVAILABLE", "message": "catalog sync configuration unavailable"})
			return
		}
		origin, ok := middleware.StrictRequestBrowserOrigin(c.Request)
		expected, err := runtime.ExternalOrigin()
		if !ok || err != nil || origin != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_ORIGIN_FORBIDDEN", "message": "request origin is not allowed"})
			return
		}
		identity, ok := middleware.GetSessionAuthIdentity(c)
		if !ok || model.ValidateCatalogRootAuthSession(c.Request.Context(), identity) != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "a current root login session is required"})
			return
		}
		c.Set("catalog_authenticated_actor", identity.UserID)
		c.Next()
	}
}

// RegisterCatalogSyncRoutes installs the entire catalog surface atomically at
// startup. Neither configuration nor the fixed source is selected by requests.
func RegisterCatalogSyncRoutes(api *gin.RouterGroup) {
	runtime, err := service.CurrentCatalogSyncRuntime()
	registerCatalogSyncRoutes(api, runtime, err, service.FetchDevCatalog)
}

type catalogSyncHTTP struct {
	runtime *service.CatalogSyncRuntime
	fetch   func(context.Context) (catalogmanifest.Snapshot, error)
}

func registerCatalogSyncRoutes(api *gin.RouterGroup, runtime *service.CatalogSyncRuntime, configurationError error, fetch func(context.Context) (catalogmanifest.Snapshot, error)) {
	h := catalogSyncHTTP{runtime: runtime, fetch: fetch}
	group := api.Group("/catalog_sync", middleware.DisableCache(), catalogSyncFailureAudit())
	group.GET("/export", catalogSyncSourceGuard(runtime, configurationError), h.export)
	group.GET("/status", middleware.RootAuth(), func(c *gin.Context) {
		// This is the sole role-aware read. Source and disabled operators still
		// need a configured browser origin and a current interactive root login.
		if configurationError != nil || runtime == nil {
			catalogSyncError(c, service.ErrCatalogSyncConfiguration)
			return
		}
		expected, err := runtime.ExternalOrigin()
		if err != nil {
			catalogSyncError(c, err)
			return
		}
		origin, ok := middleware.StrictRequestBrowserOrigin(c.Request)
		if !ok || origin != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_ORIGIN_FORBIDDEN", "message": "request origin is not allowed"})
			return
		}
		identity, ok := middleware.GetSessionAuthIdentity(c)
		if !ok || model.ValidateCatalogRootAuthSession(c.Request.Context(), identity) != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "a current root login session is required"})
			return
		}
		c.Set("catalog_authenticated_actor", identity.UserID)
		common.ApiSuccess(c, runtime.Status())
	})
	management := group.Group("", middleware.RootAuth(), catalogSyncManagementGuard(runtime, configurationError))
	management.POST("/preview", h.preview)
	management.POST("/plans/:id/resolve", h.resolve)
	management.POST("/plans/:id/apply", h.apply)
	management.GET("/history", h.history)
	management.GET("/operations/:id", h.operation)
	management.POST("/operations/:id/restore-preview", h.restorePreview)
}

// Explicit transport projections omit session/version bindings, snapshots,
// incarnations, validation programs/attestations and private durable backups.
type catalogPlanResponse struct {
	ID                 string                     `json:"id"`
	Kind               string                     `json:"kind"`
	RestoreOperationID string                     `json:"restore_operation_id,omitempty"`
	SourceID           string                     `json:"source_id"`
	TargetID           string                     `json:"target_id"`
	Digest             string                     `json:"digest"`
	ExpiresAt          int64                      `json:"expires_at"`
	Resolution         catalogmanifest.Resolution `json:"resolution"`
	Changes            []catalogmanifest.Change   `json:"changes"`
	Executable         bool                       `json:"executable"`
}

func catalogPlanSuccess(c *gin.Context, plan catalogmanifest.Plan) {
	digest, err := catalogWireDigest(plan.Digest)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	common.ApiSuccess(c, catalogPlanResponse{ID: plan.ID, Kind: plan.Kind, RestoreOperationID: plan.RestoreOperationID, SourceID: plan.Snapshot.SourceID, TargetID: plan.Actor.TargetID, Digest: digest, ExpiresAt: plan.ExpiresAt, Resolution: plan.Resolution, Changes: plan.Changes, Executable: catalogmanifest.PlanExecutable(plan, time.Now())})
}

type catalogOperationResponse struct {
	ID        string                             `json:"id"`
	PlanID    string                             `json:"plan_id"`
	State     string                             `json:"state"`
	Revision  int64                              `json:"revision"`
	CreatedAt int64                              `json:"created_at"`
	Summary   *model.CatalogSyncOperationSummary `json:"summary"`
}

func catalogOperationDTO(row model.CatalogSyncOperation) (catalogOperationResponse, error) {
	if row.Summary == nil {
		return catalogOperationResponse{}, model.ErrCatalogSyncOperationConflict
	}
	summary := *row.Summary
	digest, err := catalogWireDigest(summary.Digest)
	if err != nil {
		return catalogOperationResponse{}, err
	}
	summary.Digest = digest
	return catalogOperationResponse{row.ID, row.PlanID, row.State, row.Revision, row.CreatedAt, &summary}, nil
}

func (h catalogSyncHTTP) actor(c *gin.Context) catalogmanifest.Actor {
	identity, _ := middleware.GetSessionAuthIdentity(c)
	return catalogmanifest.Actor{UserID: identity.UserID, SessionID: identity.SessionID, AuthVersion: int(identity.UserAuthVersion), SessionVersion: int(identity.SessionVersion), TargetID: h.runtime.Status().TargetID}
}

var errCatalogRequest = errors.New("invalid catalog request")
var errCatalogBlocked = errors.New("catalog plan has unresolved blockers")

// All POSTs require a JSON object (including {} for empty requests), with exact
// case-sensitive keys, no duplicates/nulls/trailing value, and at most 64 KiB.
func catalogSyncBody(c *gin.Context, destination any, keys ...string) bool {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 65537))
	if err != nil || len(body) > 65536 || common.ValidateJsonNoDuplicateKeys(body) != nil {
		catalogSyncError(c, errCatalogRequest)
		return false
	}
	var fields map[string]common.RawMessage
	if common.Unmarshal(body, &fields) != nil || fields == nil || len(fields) != len(keys) {
		catalogSyncError(c, errCatalogRequest)
		return false
	}
	for _, key := range keys {
		raw, exists := fields[key]
		if !exists || strings.TrimSpace(string(raw)) == "null" {
			catalogSyncError(c, errCatalogRequest)
			return false
		}
	}
	if common.Unmarshal(body, destination) != nil {
		catalogSyncError(c, errCatalogRequest)
		return false
	}
	return true
}

func (h catalogSyncHTTP) export(c *gin.Context) {
	snapshot, err := model.ExportManagedCatalog(c.Request.Context(), h.runtime.Status().SourceID)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	common.ApiSuccess(c, snapshot)
}

func (h catalogSyncHTTP) preview(c *gin.Context) {
	if !catalogSyncBody(c, &struct{}{}) {
		return
	}
	snapshot, err := h.fetch(c.Request.Context())
	if err == nil && (snapshot.SourceID != h.runtime.Status().SourceID || catalogmanifest.ValidateSnapshot(snapshot) != nil) {
		err = catalogtransport.ErrUnavailable
	}
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	plan, err := model.CreateCatalogSyncPlan(c.Request.Context(), snapshot, h.actor(c), time.Now())
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	catalogPlanSuccess(c, plan)
}

func (h catalogSyncHTTP) resolve(c *gin.Context) {
	var request struct {
		Digest         string   `json:"digest"`
		OverwriteKeys  []string `json:"overwrite_keys"`
		ConfirmDeletes bool     `json:"confirm_deletes"`
	}
	if !catalogSyncBody(c, &request, "digest", "overwrite_keys", "confirm_deletes") {
		return
	}
	if !catalogDigest(request.Digest) || !catalogDigest(c.Param("id")) {
		catalogSyncError(c, errCatalogRequest)
		return
	}
	seen := make(map[string]bool, len(request.OverwriteKeys))
	for _, key := range request.OverwriteKeys {
		if key == "" || seen[key] {
			catalogSyncError(c, errCatalogRequest)
			return
		}
		seen[key] = true
	}
	plan, err := model.ResolveCatalogSyncPlan(c.Request.Context(), c.Param("id"), "sha256:"+request.Digest, h.actor(c), catalogmanifest.Resolution{OverwriteKeys: request.OverwriteKeys, ConfirmDeletes: request.ConfirmDeletes})
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	catalogPlanSuccess(c, plan)
}

// Model seals name the algorithm; proof and HTTP fields use its exact 64-byte
// lowercase hexadecimal representation. This is reversible, never a new hash.
func catalogWireDigest(value string) (string, error) {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") || !catalogDigest(value[7:]) {
		return "", model.ErrCatalogSyncPlanStale
	}
	return value[7:], nil
}

func catalogDigest(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}
func catalogOperationID(value string) bool {
	return value != "" && len(value) <= 64 && strings.TrimSpace(value) == value
}

func (h catalogSyncHTTP) apply(c *gin.Context) {
	var request struct {
		PlanID      string `json:"plan_id"`
		Digest      string `json:"digest"`
		OperationID string `json:"operation_id"`
	}
	if !catalogSyncBody(c, &request, "plan_id", "digest", "operation_id") {
		return
	}
	if !catalogDigest(request.PlanID) || !catalogDigest(request.Digest) || !catalogOperationID(request.OperationID) {
		catalogSyncError(c, errCatalogRequest)
		return
	}
	if request.PlanID != c.Param("id") {
		catalogSyncError(c, model.ErrCatalogSyncOperationConflict)
		return
	}
	c.Set("catalog_plan_id", request.PlanID)
	c.Set("catalog_operation_id", request.OperationID)
	actor := h.actor(c)
	sealedDigest := "sha256:" + request.Digest
	result, found, err := model.LookupCatalogSyncOperationResult(c.Request.Context(), request.PlanID, sealedDigest, request.OperationID, actor)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	if found {
		common.ApiSuccess(c, result)
		return
	}
	plan, err := model.GetCatalogSyncPlan(c.Request.Context(), request.PlanID, actor)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	if plan.Digest != sealedDigest || plan.Actor.TargetID != h.runtime.Status().TargetID {
		catalogSyncError(c, model.ErrCatalogSyncPlanStale)
		return
	}
	if !catalogmanifest.PlanExecutable(plan, time.Now()) {
		catalogSyncError(c, errCatalogBlocked)
		return
	}
	scope := service.VerificationScopeCatalogSyncApply
	if plan.Kind == "restore" {
		scope = service.VerificationScopeCatalogSyncRestore
	} else if plan.Kind != "sync" {
		catalogSyncError(c, model.ErrCatalogSyncPlanStale)
		return
	}
	c.Set("catalog_kind", plan.Kind)
	wireDigest, err := catalogWireDigest(plan.Digest)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	proofContext, err := common.Marshal(service.CatalogSyncVerificationContext{PlanDigest: wireDigest, TargetID: actor.TargetID, Kind: plan.Kind, OperationID: request.OperationID})
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	if len(c.Request.Header.Values("X-Security-Proof")) > 1 {
		catalogSyncError(c, errCatalogRequest)
		return
	}
	if middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: scope, Context: proofContext}) == nil {
		return
	}
	result, err = model.ApplyCatalogSyncPlan(c.Request.Context(), request.PlanID, sealedDigest, request.OperationID, actor)
	if err != nil {
		if result.OperationID != "" && !errors.Is(err, model.ErrCatalogCommitUncertain) {
			c.Set("catalog_outcome", "publication_pending")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "CATALOG_PUBLICATION_PENDING", "message": "catalog committed; query the operation before taking further action", "data": result})
			return
		}
		catalogSyncError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func (h catalogSyncHTTP) history(c *gin.Context) {
	offset, limit := 0, 20
	for key, values := range c.Request.URL.Query() {
		if (key != "offset" && key != "limit") || len(values) != 1 {
			catalogSyncError(c, errCatalogRequest)
			return
		}
		value, err := strconv.Atoi(values[0])
		if err != nil || value < 0 {
			catalogSyncError(c, errCatalogRequest)
			return
		}
		if key == "offset" {
			offset = value
		} else if value > 0 {
			limit = min(value, 100)
		}
	}
	rows, total, err := model.ListCatalogSyncOperations(c.Request.Context(), offset, limit)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	items := make([]catalogOperationResponse, 0, len(rows))
	for _, row := range rows {
		item, err := catalogOperationDTO(row)
		if err != nil {
			catalogSyncError(c, err)
			return
		}
		items = append(items, item)
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total, "offset": offset, "limit": limit})
}

func (h catalogSyncHTTP) operation(c *gin.Context) {
	if !catalogOperationID(c.Param("id")) {
		catalogSyncError(c, errCatalogRequest)
		return
	}
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		catalogSyncError(c, errCatalogRequest)
		return
	}
	if len(query) > 0 {
		if len(query) != 2 || len(query["plan_id"]) != 1 || len(query["digest"]) != 1 || !catalogDigest(query.Get("plan_id")) || !catalogDigest(query.Get("digest")) {
			catalogSyncError(c, errCatalogRequest)
			return
		}
		receipt, found, err := model.LookupCatalogSyncOperationResult(c.Request.Context(), query.Get("plan_id"), "sha256:"+query.Get("digest"), c.Param("id"), h.actor(c))
		if err != nil {
			catalogSyncError(c, err)
			return
		}
		if !found {
			catalogSyncError(c, gorm.ErrRecordNotFound)
			return
		}
		var operation *catalogOperationResponse
		if row, err := model.GetCatalogSyncOperation(c.Request.Context(), c.Param("id")); err == nil {
			if dto, err := catalogOperationDTO(row); err == nil {
				operation = &dto
			}
		}
		common.ApiSuccess(c, gin.H{"receipt": receipt, "operation": operation})
		return
	}
	row, err := model.GetCatalogSyncOperation(c.Request.Context(), c.Param("id"))
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	operation, err := catalogOperationDTO(row)
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	common.ApiSuccess(c, operation)
}

func (h catalogSyncHTTP) restorePreview(c *gin.Context) {
	if !catalogSyncBody(c, &struct{}{}) {
		return
	}
	if !catalogOperationID(c.Param("id")) {
		catalogSyncError(c, errCatalogRequest)
		return
	}
	plan, err := model.CreateCatalogSyncRestorePlan(c.Request.Context(), c.Param("id"), h.actor(c), time.Now())
	if err != nil {
		catalogSyncError(c, err)
		return
	}
	catalogPlanSuccess(c, plan)
}

func catalogSyncError(c *gin.Context, err error) {
	status, code, message := http.StatusServiceUnavailable, "CATALOG_SYNC_UNAVAILABLE", "catalog sync unavailable"
	switch {
	case errors.Is(err, model.ErrCatalogCommitUncertain):
		code, message = "CATALOG_COMMIT_UNKNOWN", "commit outcome unknown; query the same operation ID, do not automatically retry"
	case errors.Is(err, errCatalogRequest):
		status, code, message = http.StatusBadRequest, "CATALOG_REQUEST_INVALID", "invalid catalog request"
	case errors.Is(err, errCatalogBlocked):
		status, code, message = http.StatusUnprocessableEntity, "CATALOG_PLAN_BLOCKED", "catalog plan has unresolved blockers"
	case errors.Is(err, model.ErrCatalogSyncOperationConflict), errors.Is(err, model.ErrCatalogSyncPlanStale), errors.Is(err, model.ErrCatalogSyncPlanUnavailable):
		status, code, message = http.StatusConflict, "CATALOG_CONFLICT", "catalog plan or operation changed; reload before confirming"
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, code, message = http.StatusNotFound, "CATALOG_NOT_FOUND", "catalog plan or operation unavailable"
	case errors.Is(err, catalogtransport.ErrUnavailable):
		code, message = "CATALOG_SOURCE_UNAVAILABLE", "catalog source unavailable"
	case errors.Is(err, model.ErrCatalogPublicationPending):
		code, message = "CATALOG_PUBLICATION_PENDING", "catalog publication pending; query the operation"
	}
	c.Set("catalog_outcome", code)
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": message})
}

func catalogSyncFailureAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Catalog successes already have atomic engine history; failures need
		// the narrower outcome projection below, including unknown COMMIT.
		common.SetContextKey(c, constant.ContextKeyAuditLogged, true)
		c.Next()
		if c.Writer.Status() < 400 {
			return
		}
		outcome := c.GetString("catalog_outcome")
		if outcome == "" {
			outcome = "request_denied"
		}
		// Pass no request: untrusted User-Agent/Request-ID/forwarded headers must
		// not smuggle credentials into an otherwise sanitized audit record.
		entry := model.AuditLog{Category: model.AuditCategorySecurity, Action: "catalog.sync.attempt", Username: "unknown", AuthMethod: "unknown", Method: c.Request.Method, Route: c.FullPath(), Status: c.Writer.Status(), Content: outcome, Success: false}
		if id := c.GetInt("catalog_authenticated_actor"); id > 0 {
			entry.UserId, entry.ActorRole, entry.AuthMethod, entry.Username = id, common.RoleRootUser, "session", "root"
		}
		for _, key := range []string{"catalog_plan_id", "catalog_operation_id"} {
			if value := c.GetString(key); value != "" {
				entry.Content += fmt.Sprintf(" %s_sha256=%x", key, sha256.Sum256([]byte(value)))
			}
		}
		if kind := c.GetString("catalog_kind"); kind == "sync" || kind == "restore" {
			entry.Content += " kind=" + kind
		}
		model.RecordAuditLog(nil, entry)
	}
}

// CatalogSyncSourceGuard grants only a GET export privilege, without dashboard
// identity, role or proof. Capture the process singleton, preserving counters
// across requests and guard instances; protected rotation requires restart.
func CatalogSyncSourceGuard() gin.HandlerFunc {
	runtime, err := service.CurrentCatalogSyncRuntime()
	return catalogSyncSourceGuard(runtime, err)
}

func catalogSyncSourceGuard(runtime *service.CatalogSyncRuntime, configurationError error) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		token := ""
		if len(values) == 1 {
			parts := strings.Split(values[0], " ")
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}
		if configurationError != nil || runtime == nil || !runtime.Status().SourceReady || c.Request.Method != http.MethodGet || runtime.AuthenticateReader(token) != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "CATALOG_SYNC_READER_DENIED", "message": "catalog sync reader denied"})
			return
		}
		c.Next()
	}
}
