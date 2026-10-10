//go:build catalogsynctest

package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/internal/catalogtransport"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func catalogHTTPRequest(router http.Handler, method, path, access, proof, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://example.com/api/catalog_sync"+path, strings.NewReader(body))
	r.Header.Set("Origin", "https://example.com")
	r.Header.Set("Content-Type", "application/json")
	if access != "" {
		r.Header.Set("Authorization", "Bearer "+access)
	}
	if proof != "" {
		r.Header.Set("X-Security-Proof", proof)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func catalogHTTPSource(t *testing.T) ([]byte, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := catalogSyncPostgres(t)
	require.NoError(t, db.Create(&model.Vendor{Name: "http-vendor", Description: "raw source description", Status: 1}).Error)
	env, token := catalogSourceEnvironment(t)
	runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	router := gin.New()
	registerCatalogSyncRoutes(router.Group("/api"), runtime, nil, service.FetchDevCatalog)
	response := catalogHTTPRequest(router, "GET", "/export", token, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	return append([]byte(nil), response.Body.Bytes()...), token
}

func catalogHTTPFetch(t *testing.T, body []byte, token string) func(context.Context) (catalogmanifest.Snapshot, error) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"dev.molii.co"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/catalog_sync/export", r.URL.Path)
		assert.Equal(t, "dev.molii.co", r.Host)
		assert.Equal(t, "dev.molii.co", r.TLS.ServerName)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	return func(ctx context.Context) (catalogmanifest.Snapshot, error) {
		return catalogtransport.FetchControlledForHTTPTest(ctx, token, "dev", func(ctx context.Context, host string) ([]netip.Addr, error) {
			assert.Equal(t, "dev.molii.co", host)
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}, func(ctx context.Context, network, address string) (net.Conn, error) {
			assert.Equal(t, "93.184.216.34:443", address)
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}, roots)
	}
}

func catalogHTTPTarget(t *testing.T, fetch func(context.Context) (catalogmanifest.Snapshot, error)) (*gorm.DB, *gin.Engine, service.AuthIdentity, string) {
	t.Helper()
	db, actor, _ := catalogSyncTarget(t)
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	priorLog, priorLogType, priorSecret, priorPassword := model.LOG_DB, common.LogDatabaseType(), common.SessionSecret, common.PasswordLoginEnabled
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	common.SessionSecret, common.PasswordLoginEnabled = "http-catalog-task-only", true
	t.Cleanup(func() {
		model.LOG_DB = priorLog
		common.SetLogDatabaseType(priorLogType)
		common.SessionSecret, common.PasswordLoginEnabled = priorSecret, priorPassword
	})
	require.NoError(t, db.AutoMigrate(&model.TwoFA{}, &model.TwoFABackupCode{}, &model.PasskeyCredential{}, &model.AuthFlow{}, &model.UserOAuthBinding{}, &model.Token{}, &model.AuditLog{}))
	password, err := common.Password2Hash("catalog-test-password")
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", actor.UserID).Update("password", password).Error)
	identity := service.AuthIdentity{UserID: actor.UserID, SessionID: actor.SessionID, UserAuthVersion: int64(actor.AuthVersion), SessionVersion: int64(actor.SessionVersion)}
	access, _, err := service.IssueAccessToken(identity)
	require.NoError(t, err)
	router := gin.New()
	registerCatalogSyncRoutes(router.Group("/api"), catalogManagementRuntime(t), nil, fetch)
	return db, router, identity, access
}

func catalogHTTPPlan(t *testing.T, response *httptest.ResponseRecorder) catalogPlanResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var envelope struct {
		Success bool                `json:"success"`
		Data    catalogPlanResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.NotEmpty(t, envelope.Data.ID)
	return envelope.Data
}

func catalogHTTPApplyBody(t *testing.T, plan catalogPlanResponse, operation string) string {
	t.Helper()
	body, err := common.Marshal(map[string]string{"plan_id": plan.ID, "digest": plan.Digest, "operation_id": operation})
	require.NoError(t, err)
	return string(body)
}

func catalogHTTPProof(t *testing.T, identity service.AuthIdentity, plan catalogPlanResponse, operation string) (string, service.VerificationOperation) {
	t.Helper()
	scope := service.VerificationScopeCatalogSyncApply
	if plan.Kind == "restore" {
		scope = service.VerificationScopeCatalogSyncRestore
	}
	raw, err := common.Marshal(service.CatalogSyncVerificationContext{PlanDigest: plan.Digest, TargetID: "target", Kind: plan.Kind, OperationID: operation})
	require.NoError(t, err)
	op := service.VerificationOperation{Scope: scope, Context: raw}
	proof, err := service.VerifySecurityInput(identity, service.VerificationInput{Scope: op.Scope, Context: op.Context, Method: "password", Password: "catalog-test-password"})
	require.NoError(t, err)
	return proof.ProofToken, op
}

func TestCatalogSyncHTTPBusinessAndReceiptQuery(t *testing.T) {
	body, token := catalogHTTPSource(t)
	db, router, identity, access := catalogHTTPTarget(t, catalogHTTPFetch(t, body, token))
	plan := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`))
	require.True(t, plan.Executable)
	assert.NotEmpty(t, plan.Changes)
	assert.Equal(t, 403, catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, "", catalogHTTPApplyBody(t, plan, "http-operation")).Code)
	proof, _ := catalogHTTPProof(t, identity, plan, "http-operation")
	requestBody := catalogHTTPApplyBody(t, plan, "http-operation")
	response := catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, requestBody)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "committed_pending_publish")
	var vendor model.Vendor
	require.NoError(t, db.Where("name = ?", "http-vendor").First(&vendor).Error)
	assert.Equal(t, "raw source description", vendor.Description)
	query := "/operations/http-operation?plan_id=" + plan.ID + "&digest=" + plan.Digest
	got := catalogHTTPRequest(router, "GET", query, access, "", "")
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	assert.Contains(t, got.Body.String(), `"receipt":{"operation_id":"http-operation","state":"committed_pending_publish"`)
	assert.Contains(t, got.Body.String(), `"state":"succeeded"`)
	otherSession, err := service.CreateLoginSession(identity.UserID, "password", "127.0.0.1", "http-other-session")
	require.NoError(t, err)
	assert.Equal(t, 409, catalogHTTPRequest(router, "GET", query, otherSession.AccessToken, "", "").Code, "an exact receipt remains bound to its original SID")
	for _, private := range []string{identity.SessionID, "auth_version", "session_version", "backup", "object_versions", "validation_digest", token} {
		assert.NotContains(t, got.Body.String(), private)
	}
	require.NoError(t, db.Where("id = ?", plan.ID).Delete(&model.CatalogSyncPlan{}).Error)
	require.NoError(t, db.Exec("DROP TABLE marketplace_order_locks").Error)
	replay := catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, "", requestBody)
	require.Equal(t, http.StatusOK, replay.Code, replay.Body.String())
	assert.JSONEq(t, response.Body.String(), replay.Body.String())
	got = catalogHTTPRequest(router, "GET", query, access, "", "")
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	assert.Contains(t, got.Body.String(), `"operation":null`)
	assert.Contains(t, got.Body.String(), `"operation_id":"http-operation"`)
	var count int64
	require.NoError(t, db.Model(&model.CatalogSyncOperation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCatalogSyncHTTPStrictBodies(t *testing.T) {
	_, _, access := setupCatalogSecurityTest(t)
	router := gin.New()
	registerCatalogSyncRoutes(router.Group("/api"), catalogManagementRuntime(t), nil, func(context.Context) (catalogmanifest.Snapshot, error) {
		t.Fatal("invalid request must not fetch source")
		return catalogmanifest.Snapshot{}, nil
	})
	for _, body := range []string{"", `null`, `[]`, `{"snapshot":{}}`, `{} {}`, `{"x":1,"x":2}`, strings.Repeat(" ", 65536) + `{}`, "{\"x\":\"\xff\"}"} {
		assert.Equal(t, http.StatusBadRequest, catalogHTTPRequest(router, "POST", "/preview", access, "", body).Code)
	}
	for _, body := range []string{`{"plan_id":"x","digest":"x","operation_id":"x","actor":1}`, `{"PLAN_ID":"x","digest":"x","operation_id":"x"}`, `{"plan_id":null,"digest":"x","operation_id":"x"}`, `{"plan_id":1,"digest":"x","operation_id":"x"}`, `{"plan_id":"x","digest":"x","operation_id":"x"}`} {
		assert.Equal(t, http.StatusBadRequest, catalogHTTPRequest(router, "POST", "/plans/x/apply", access, "", body).Code)
	}
	for _, query := range []string{"?plan_id=x", "?digest=x", "?foo=x", "?plan_id=" + strings.Repeat("a", 64) + "&digest=" + strings.Repeat("b", 64) + "&digest=" + strings.Repeat("c", 64)} {
		assert.Equal(t, http.StatusBadRequest, catalogHTTPRequest(router, "GET", "/operations/op"+query, access, "", "").Code)
	}
	for _, keys := range []string{`[null]`, `[""]`, `["vendor:x","vendor:x"]`} {
		body := fmt.Sprintf(`{"digest":%q,"overwrite_keys":%s,"confirm_deletes":false}`, strings.Repeat("a", 64), keys)
		assert.Equal(t, 400, catalogHTTPRequest(router, "POST", "/plans/"+strings.Repeat("a", 64)+"/resolve", access, "", body).Code)
	}
}

func TestCatalogSyncHTTPUnavailableSourceCannotDelete(t *testing.T) {
	for _, body := range []string{`{"success":false,"message":"upstream secret","data":null}`, `{"success":true,"message":"","data":`, `{}`} {
		t.Run(body, func(t *testing.T) {
			db, router, _, access := catalogHTTPTarget(t, catalogHTTPFetch(t, []byte(body), "dummy-test-only"))
			var before, after int64
			require.NoError(t, db.Model(&model.CatalogSyncPlan{}).Count(&before).Error)
			response := catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`)
			assert.Equal(t, http.StatusServiceUnavailable, response.Code)
			assert.Contains(t, response.Body.String(), "CATALOG_SOURCE_UNAVAILABLE")
			assert.NotContains(t, response.Body.String(), "upstream secret")
			require.NoError(t, db.Model(&model.CatalogSyncPlan{}).Count(&after).Error)
			assert.Equal(t, before, after)
		})
	}
}

func TestCatalogSyncHTTPNetworkOutageCannotDelete(t *testing.T) {
	fetch := func(ctx context.Context) (catalogmanifest.Snapshot, error) {
		return catalogtransport.FetchControlledForHTTPTest(ctx, "task-only-reader", "dev", func(context.Context, string) ([]netip.Addr, error) { return nil, errors.New("private resolver detail") }, func(context.Context, string, string) (net.Conn, error) {
			t.Error("failed DNS must not dial")
			return nil, errors.New("unreachable")
		}, nil)
	}
	db, router, _, access := catalogHTTPTarget(t, fetch)
	var before, after int64
	require.NoError(t, db.Model(&model.CatalogSyncPlan{}).Count(&before).Error)
	response := catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`)
	assert.Equal(t, 503, response.Code)
	assert.Contains(t, response.Body.String(), "CATALOG_SOURCE_UNAVAILABLE")
	assert.NotContains(t, response.Body.String(), "private resolver")
	require.NoError(t, db.Model(&model.CatalogSyncPlan{}).Count(&after).Error)
	assert.Equal(t, before, after)
}

func TestCatalogSyncHTTPProofOrderingAndRestore(t *testing.T) {
	body, token := catalogHTTPSource(t)
	db, router, identity, access := catalogHTTPTarget(t, catalogHTTPFetch(t, body, token))
	plan := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`))
	proof, operation := catalogHTTPProof(t, identity, plan, "sync-one")
	for _, mismatch := range []string{"digest", "operation", "scope", "target", "kind"} {
		t.Run(mismatch, func(t *testing.T) {
			currentPlan, opID := plan, "sync-one"
			if mismatch == "digest" {
				currentPlan.Digest = strings.Repeat("f", 64)
			}
			if mismatch == "operation" {
				opID = "wrong-operation"
			}
			candidateProof := proof
			if mismatch == "scope" || mismatch == "target" || mismatch == "kind" {
				ctx := service.CatalogSyncVerificationContext{PlanDigest: plan.Digest, TargetID: "target", Kind: "sync", OperationID: "sync-one"}
				scope := service.VerificationScopeCatalogSyncApply
				if mismatch == "target" {
					ctx.TargetID = "other"
				} else {
					ctx.Kind, scope = "restore", service.VerificationScopeCatalogSyncRestore
				}
				raw, err := common.Marshal(ctx)
				require.NoError(t, err)
				issued, err := service.VerifySecurityInput(identity, service.VerificationInput{Scope: scope, Context: raw, Method: "password", Password: "catalog-test-password"})
				require.NoError(t, err)
				candidateProof = issued.ProofToken
			}
			response := catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, candidateProof, catalogHTTPApplyBody(t, currentPlan, opID))
			if mismatch == "digest" {
				assert.Equal(t, http.StatusConflict, response.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
			}
		})
	}
	// A definite model failure after proof consumption must burn it permanently.
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("http-definite-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "catalog_sync_operations" {
			tx.AddError(errors.New("private-db-detail-do-not-expose"))
		}
	}))
	response := catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, catalogHTTPApplyBody(t, plan, "sync-one"))
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.NotContains(t, response.Body.String(), "private-db-detail")
	require.NoError(t, db.Callback().Create().Remove("http-definite-failure"))
	_, err := service.ConsumeOperationProof(proof, identity, operation)
	assert.ErrorIs(t, err, service.ErrProofConsumed)
	var count int64
	require.NoError(t, db.Model(&model.CatalogSyncOperation{}).Count(&count).Error)
	assert.Zero(t, count)
	proof, _ = catalogHTTPProof(t, identity, plan, "sync-one")
	response = catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, catalogHTTPApplyBody(t, plan, "sync-one"))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	restored := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/operations/sync-one/restore-preview", access, "", `{}`))
	assert.Equal(t, "restore", restored.Kind)
	resolveBody := fmt.Sprintf(`{"digest":%q,"overwrite_keys":[],"confirm_deletes":true}`, restored.Digest)
	restored = catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/plans/"+restored.ID+"/resolve", access, "", resolveBody))
	require.True(t, restored.Executable)
	wrong := restored
	wrong.Kind = "sync"
	wrongProof, _ := catalogHTTPProof(t, identity, wrong, "restore-one")
	response = catalogHTTPRequest(router, "POST", "/plans/"+restored.ID+"/apply", access, wrongProof, catalogHTTPApplyBody(t, restored, "restore-one"))
	assert.Equal(t, http.StatusForbidden, response.Code)
	restoreProof, _ := catalogHTTPProof(t, identity, restored, "restore-one")
	response = catalogHTTPRequest(router, "POST", "/plans/"+restored.ID+"/apply", access, restoreProof, catalogHTTPApplyBody(t, restored, "restore-one"))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NoError(t, db.Model(&model.Vendor{}).Where("name = ?", "http-vendor").Count(&count).Error)
	assert.Zero(t, count, "actual inverse removes the source-created vendor")
	response = catalogHTTPRequest(router, "GET", "/history?limit=200", access, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"limit":100`)
	assert.Contains(t, response.Body.String(), `"restore_operation_id":"sync-one"`)
	assert.NotContains(t, response.Body.String(), identity.SessionID)
	assert.NotContains(t, response.Body.String(), "sha256:")
	response = catalogHTTPRequest(router, "POST", "/operations/sync-one/restore-preview", access, "", `{}`)
	assert.Equal(t, http.StatusConflict, response.Code, "only latest operation is restorable")
	response = catalogHTTPRequest(router, "POST", "/plans/"+restored.ID+"/apply", access, "", catalogHTTPApplyBody(t, restored, "restore-one"))
	assert.Equal(t, http.StatusOK, response.Code, "restore exact replay needs no new proof")
}

func TestCatalogSyncHTTPPendingReplayAndUnknownCommit(t *testing.T) {
	for _, fault := range []string{"publish", "commit"} {
		t.Run(fault, func(t *testing.T) {
			body, token := catalogHTTPSource(t)
			db, router, identity, access := catalogHTTPTarget(t, catalogHTTPFetch(t, body, token))
			plan := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`))
			proof, _ := catalogHTTPProof(t, identity, plan, "fault-operation")
			var response *httptest.ResponseRecorder
			if fault == "publish" {
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("http-publish-fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["state"] == "succeeded" {
							tx.AddError(errors.New("private-publish-fault"))
						}
					}
				}))
				response = catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, catalogHTTPApplyBody(t, plan, "fault-operation"))
				require.NoError(t, db.Callback().Update().Remove("http-publish-fault"))
				assert.Contains(t, response.Body.String(), "CATALOG_PUBLICATION_PENDING")
				assert.Contains(t, response.Body.String(), `"operation_id":"fault-operation"`)
			} else {
				mutated, lost := false, false
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("http-commit-start", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						mutated = true
					}
				}))
				catalogSyncObserveTarget(t, db, func(connection *catalogReadLostAckConnection) {
					connection.afterCommit = func() error {
						if mutated && !lost {
							lost = true
							return errors.New("private-lost-commit")
						}
						return nil
					}
					connection.beforeSQL = func(_ context.Context, query string) error {
						if lost && strings.Contains(query, "catalog_sync_operations") {
							return errors.New("private-lookup-outage")
						}
						return nil
					}
					response = catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, catalogHTTPApplyBody(t, plan, "fault-operation"))
				})
				require.NoError(t, db.Callback().Create().Remove("http-commit-start"))
				require.True(t, lost, "actual mutation COMMIT lost its acknowledgment")
				assert.Contains(t, response.Body.String(), "CATALOG_COMMIT_UNKNOWN")
			}
			require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), "private-")
			var before model.CatalogSyncState
			require.NoError(t, db.First(&before).Error)
			require.Equal(t, "committed_pending_publish", before.PublicationState)
			query := "/operations/fault-operation?plan_id=" + plan.ID + "&digest=" + plan.Digest
			response = catalogHTTPRequest(router, "GET", query, access, "", "")
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), `"operation_id":"fault-operation"`)
			response = catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, "", catalogHTTPApplyBody(t, plan, "fault-operation"))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var after model.CatalogSyncState
			require.NoError(t, db.First(&after).Error)
			assert.Equal(t, before, after, "query and replay must not automatically recover or mutate")
			var count int64
			require.NoError(t, db.Model(&model.CatalogSyncOperation{}).Count(&count).Error)
			assert.Equal(t, int64(1), count)
			var audit []model.AuditLog
			require.NoError(t, db.Find(&audit).Error)
			require.NotEmpty(t, audit)
			require.Len(t, audit, 1, "unknown/publication failure must have one dedicated outcome, no generic duplicate")
			assert.Equal(t, "catalog.sync.attempt", audit[0].Action)
			encoded, err := common.Marshal(audit)
			require.NoError(t, err)
			for _, secret := range []string{proof, access, token, identity.SessionID, "private-"} {
				assert.NotContains(t, string(encoded), secret)
			}
			if fault == "commit" {
				assert.Contains(t, string(encoded), "CATALOG_COMMIT_UNKNOWN")
			}
		})
	}
}

func TestCatalogSyncHTTPConcurrentProofAndExactBinding(t *testing.T) {
	body, token := catalogHTTPSource(t)
	db, router, identity, access := catalogHTTPTarget(t, catalogHTTPFetch(t, body, token))
	plan := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`))
	proof, _ := catalogHTTPProof(t, identity, plan, "concurrent-operation")
	request := catalogHTTPApplyBody(t, plan, "concurrent-operation")
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses <- catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, request)
		}()
	}
	wg.Wait()
	close(responses)
	success := 0
	for response := range responses {
		if response.Code == http.StatusOK {
			success++
		} else {
			assert.Contains(t, []int{http.StatusForbidden, http.StatusServiceUnavailable, http.StatusConflict}, response.Code, response.Body.String())
		}
	}
	assert.Positive(t, success)
	var count int64
	require.NoError(t, db.Model(&model.CatalogSyncOperation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	wrong := plan
	wrong.Digest = strings.Repeat("f", 64)
	response := catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, "", catalogHTTPApplyBody(t, wrong, "concurrent-operation"))
	assert.Equal(t, http.StatusConflict, response.Code)
	query := "/operations/concurrent-operation?plan_id=" + plan.ID + "&digest=" + wrong.Digest
	assert.Equal(t, http.StatusConflict, catalogHTTPRequest(router, "GET", query, access, "", "").Code)
}

func TestCatalogSyncHTTPAuthOriginAndAudit(t *testing.T) {
	user, identity, access := setupCatalogSecurityTest(t)
	router := gin.New()
	registerCatalogSyncRoutes(router.Group("/api"), catalogManagementRuntime(t), nil, service.FetchDevCatalog)
	require.NoError(t, model.UpdateUserAccessToken(user.Id, "http-root-pat"))
	require.NoError(t, model.DB.Create(&model.Token{UserId: user.Id, Key: "http-relay-token", Status: common.TokenStatusEnabled, Name: "http-test"}).Error)
	for _, route := range []struct{ method, path, body string }{{"GET", "/status", ""}, {"POST", "/preview", `{}`}, {"POST", "/plans/id/resolve", `{}`}, {"POST", "/plans/id/apply", `{}`}, {"GET", "/history", ""}, {"GET", "/operations/id", ""}, {"POST", "/operations/id/restore-preview", `{}`}} {
		for _, credential := range []string{"", "http-root-pat", "http-relay-token", "reader-a." + strings.Repeat("a", 43)} {
			response := catalogHTTPRequest(router, route.method, route.path, credential, "", route.body)
			assert.Contains(t, []int{401, 403}, response.Code, response.Body.String())
		}
	}
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		require.NoError(t, model.DB.Model(user).Update("role", role).Error)
		assert.Equal(t, 403, catalogHTTPRequest(router, "GET", "/status", access, "", "").Code)
	}
	require.NoError(t, model.DB.Model(user).Update("role", common.RoleRootUser).Error)
	priorSecure := common.SessionCookieSecure
	common.SessionCookieSecure = false
	t.Cleanup(func() { common.SessionCookieSecure = priorSecure })
	for _, test := range []struct {
		origin, referer string
		want            int
	}{{"https://example.com", "", 200}, {"", "https://example.com/settings", 200}, {"", "", 403}, {"null", "https://example.com/", 403}, {"https://attacker.example", "https://example.com/", 403}, {"https://example.com/", "", 403}, {"https://example.com:", "", 403}} {
		request := httptest.NewRequest("GET", "http://attacker-proxy/api/catalog_sync/status", nil)
		request.Header.Set("Authorization", "Bearer "+access)
		request.Header.Set("Forwarded", "host=example.com;proto=https")
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("X-Forwarded-Host", "example.com")
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		if test.referer != "" {
			request.Header.Set("Referer", test.referer)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, test.want, response.Code)
	}
	request := httptest.NewRequest("POST", "https://example.com/api/catalog_sync/plans/private-path-secret/apply", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+access)
	request.Header.Set("Origin", "https://example.com")
	request.Header.Set("User-Agent", "private-user-agent-secret")
	request.Header.Set("X-Request-ID", "private-request-id-secret")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, 400, response.Code)
	var logs []model.AuditLog
	require.NoError(t, model.DB.Find(&logs).Error)
	encoded, err := common.Marshal(logs)
	require.NoError(t, err)
	for _, secret := range []string{"private-path-secret", "private-user-agent-secret", "private-request-id-secret", identity.SessionID, access} {
		assert.NotContains(t, string(encoded), secret)
	}
	for _, entry := range logs {
		if entry.Action == "catalog.sync.attempt" && entry.AuthMethod == "unknown" {
			assert.Zero(t, entry.UserId)
			assert.Zero(t, entry.ActorRole)
		}
	}
}

func TestCatalogSyncHTTPSourceAndMissingOrigin(t *testing.T) {
	_, _, access := setupCatalogSecurityTest(t)
	env, token := catalogSourceEnvironment(t)
	runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	router := gin.New()
	registerCatalogSyncRoutes(router.Group("/api"), runtime, nil, service.FetchDevCatalog)
	assert.Equal(t, 503, catalogHTTPRequest(router, "GET", "/status", access, "", "").Code)
	for _, invalid := range []string{"", access, token + "x"} {
		assert.Equal(t, 401, catalogHTTPRequest(router, "GET", "/export", invalid, "", "").Code)
	}
	for range 59 {
		require.Equal(t, 200, catalogHTTPRequest(router, "GET", "/export", token, "", "").Code)
	}
	// The earlier bad-secret attempt used this reader's fixed 60-attempt budget.
	assert.Equal(t, 401, catalogHTTPRequest(router, "GET", "/export", token, "", "").Code)
	assert.Equal(t, 404, catalogHTTPRequest(router, "POST", "/export", token, "", `{}`).Code)
	assert.Equal(t, 401, catalogHTTPRequest(router, "POST", "/preview", token, "", `{}`).Code)
	env["CATALOG_SYNC_READERS_JSON"] = strings.ReplaceAll(env["CATALOG_SYNC_READERS_JSON"], "active", "revoked")
	runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	revoked := gin.New()
	registerCatalogSyncRoutes(revoked.Group("/api"), runtime, nil, service.FetchDevCatalog)
	assert.Equal(t, 401, catalogHTTPRequest(revoked, "GET", "/export", token, "", "").Code)
}

func TestCatalogSyncHTTPAuthoritativeSessions(t *testing.T) {
	for _, change := range []string{"revoked", "expired", "demoted", "session-version", "user-version"} {
		t.Run(change, func(t *testing.T) {
			user, identity, access := setupCatalogSecurityTest(t)
			router := gin.New()
			registerCatalogSyncRoutes(router.Group("/api"), catalogManagementRuntime(t), nil, service.FetchDevCatalog)
			require.Equal(t, 200, catalogHTTPRequest(router, "GET", "/status", access, "", "").Code)
			session := model.DB.Model(&model.UserSession{}).Where("sid = ?", identity.SessionID)
			switch change {
			case "revoked":
				require.NoError(t, session.Update("revoked_at", time.Now().Unix()).Error)
			case "expired":
				require.NoError(t, session.Update("expires_at", time.Now().Unix()).Error)
			case "demoted":
				require.NoError(t, model.DB.Model(user).Update("role", common.RoleAdminUser).Error)
			case "session-version":
				require.NoError(t, session.Update("version", 2).Error)
			case "user-version":
				require.NoError(t, model.DB.Model(user).Update("auth_version", 2).Error)
			}
			response := catalogHTTPRequest(router, "GET", "/status", access, "", "")
			assert.Contains(t, []int{401, 403}, response.Code)
		})
	}
}

func TestCatalogSyncHTTPDigestRepresentation(t *testing.T) {
	for _, invalid := range []string{strings.Repeat("a", 64), "sha512:" + strings.Repeat("a", 64), "SHA256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("A", 64), "sha256:" + strings.Repeat("a", 63)} {
		_, err := catalogWireDigest(invalid)
		assert.Error(t, err)
	}
	got, err := catalogWireDigest("sha256:" + strings.Repeat("a", 64))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 64), got)
}

func TestCatalogSyncHTTPBlockResolveExpiryAndProofDeadline(t *testing.T) {
	body, token := catalogHTTPSource(t)
	db, router, identity, access := catalogHTTPTarget(t, catalogHTTPFetch(t, body, token))
	initial := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`))
	initialProof, _ := catalogHTTPProof(t, identity, initial, "initial-baseline")
	initialResponse := catalogHTTPRequest(router, "POST", "/plans/"+initial.ID+"/apply", access, initialProof, catalogHTTPApplyBody(t, initial, "initial-baseline"))
	require.Equal(t, 200, initialResponse.Code, initialResponse.Body.String())
	require.NoError(t, db.Model(&model.Vendor{}).Where("name = ?", "http-vendor").Update("description", "target local value").Error)
	plan := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/preview", access, "", `{}`))
	require.False(t, plan.Executable)
	proof, op := catalogHTTPProof(t, identity, plan, "blocked-operation")
	response := catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, catalogHTTPApplyBody(t, plan, "blocked-operation"))
	assert.Equal(t, 422, response.Code, response.Body.String())
	_, err := service.ConsumeOperationProof(proof, identity, op)
	require.NoError(t, err, "blocked precheck must leave proof unconsumed")
	var units []string
	for _, change := range plan.Changes {
		if change.Action == "conflict" {
			units = append(units, catalogmanifest.ConfirmationUnit(change))
		}
	}
	require.NotEmpty(t, units)
	request, err := common.Marshal(map[string]any{"digest": plan.Digest, "overwrite_keys": units, "confirm_deletes": false})
	require.NoError(t, err)
	resolved := catalogHTTPPlan(t, catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/resolve", access, "", string(request)))
	assert.NotEqual(t, plan.Digest, resolved.Digest)
	require.True(t, resolved.Executable)
	assert.Equal(t, 409, catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/resolve", access, "", string(request)).Code, "stale browser resolve cannot overwrite final choices")
	proof, _ = catalogHTTPProof(t, identity, resolved, "deadline-operation")
	require.NoError(t, db.Model(&model.AuthFlow{}).Where("consumed_at IS NULL").Update("expires_at", time.Now().Add(-time.Second)).Error)
	response = catalogHTTPRequest(router, "POST", "/plans/"+plan.ID+"/apply", access, proof, catalogHTTPApplyBody(t, resolved, "deadline-operation"))
	assert.Equal(t, 403, response.Code, response.Body.String())
	var source struct {
		Data catalogmanifest.Snapshot `json:"data"`
	}
	require.NoError(t, common.Unmarshal(body, &source))
	actor := catalogmanifest.Actor{UserID: identity.UserID, SessionID: identity.SessionID, AuthVersion: int(identity.UserAuthVersion), SessionVersion: int(identity.SessionVersion), TargetID: "target"}
	expired, err := model.CreateCatalogSyncPlan(context.Background(), source.Data, actor, time.Now().Add(-11*time.Minute))
	require.NoError(t, err)
	assert.Less(t, expired.ExpiresAt, time.Now().Unix())
	expiredDigest, err := catalogWireDigest(expired.Digest)
	require.NoError(t, err)
	expiredDTO := catalogPlanResponse{ID: expired.ID, Digest: expiredDigest, Kind: expired.Kind}
	proof, op = catalogHTTPProof(t, identity, expiredDTO, "expired-operation")
	response = catalogHTTPRequest(router, "POST", "/plans/"+expired.ID+"/apply", access, proof, catalogHTTPApplyBody(t, expiredDTO, "expired-operation"))
	assert.Equal(t, 409, response.Code, response.Body.String())
	_, err = service.ConsumeOperationProof(proof, identity, op)
	require.NoError(t, err, "expired plan precheck must not burn proof")
	var count int64
	require.NoError(t, db.Model(&model.CatalogSyncOperation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "only the earlier genuine baseline operation exists")
}

func TestCatalogSyncHTTPStatusRoles(t *testing.T) {
	_, _, access := setupCatalogSecurityTest(t)
	for _, role := range []string{"source", "disabled", "target"} {
		t.Run(role, func(t *testing.T) {
			env, _ := catalogSourceEnvironment(t)
			env["CATALOG_SYNC_EXTERNAL_ORIGIN"] = "https://example.com"
			var runtime *service.CatalogSyncRuntime
			if role == "target" {
				runtime = catalogManagementRuntime(t)
			} else {
				if role == "disabled" {
					env = map[string]string{"CATALOG_SYNC_EXTERNAL_ORIGIN": "https://example.com"}
				}
				var err error
				runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
				require.NoError(t, err)
			}
			router := gin.New()
			registerCatalogSyncRoutes(router.Group("/api"), runtime, nil, service.FetchDevCatalog)
			response := catalogHTTPRequest(router, "GET", "/status", access, "", "")
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var envelope struct {
				Success bool                      `json:"success"`
				Data    service.CatalogSyncStatus `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
			assert.True(t, envelope.Success)
			assert.Equal(t, role, envelope.Data.Role)
			if role != "target" {
				for _, path := range []string{"/preview", "/plans/plan/apply"} {
					assert.Equal(t, http.StatusServiceUnavailable, catalogHTTPRequest(router, "POST", path, access, "", `{}`).Code)
				}
			}
		})
	}
}
