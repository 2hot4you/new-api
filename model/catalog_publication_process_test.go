package model

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A second process owns independent Go locks and runtime caches but shares the
// parent's disposable PostgreSQL catalog. Its unchanged refresh must not close
// the shared gate while the first process is selecting its published price.
func TestCatalogPublicationProcesses(t *testing.T) {
	const processKey = "CCG_CATALOG_PUBLICATION_PROCESS"
	if os.Getenv(processKey) == "child" {
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		require.NoError(t, catalogPostgresDSN(dsn))
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(u.Path, "/catalog_sync_"), "child may only open the parent's owned fixture database")
		db, err := gorm.Open(postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: dsn, PreferSimpleProtocol: true}}}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		defer pool.Close()
		DB, LOG_DB = db, db
		common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
		common.RedisEnabled, common.MemoryCacheEnabled = false, false
		jsplugin.DefaultRegistry = jsplugin.NewRegistry()
		initCol()
		require.NoError(t, InitOptionMapBootstrap(context.Background()))
		require.NoError(t, CompleteOptionMapInitialization(context.Background()))
		fmt.Println("catalog-process:ready")
		input := bufio.NewReader(os.Stdin)
		_, err = input.ReadString('\n')
		require.NoError(t, err)
		if os.Getenv("CCG_CATALOG_PUBLICATION_DRIFT") != "noop" {
			require.NoError(t, db.Save(&Option{Key: "USDExchangeRate", Value: "14"}).Error)
		}
		var capture sync.Once
		require.NoError(t, db.Callback().Query().After("gorm:query").Register("process-refresh-captured", func(tx *gorm.DB) {
			if tx.Statement.Table == "options" && tx.Statement.ConnPool == db.Statement.ConnPool {
				capture.Do(func() {
					fmt.Println("catalog-process:captured")
					_, err := input.ReadString('\n')
					tx.AddError(err)
				})
			}
		}))
		require.NoError(t, loadOptionsFromDatabase())
		fmt.Println("catalog-process:done")
		return
	}

	for _, drift := range []string{"noop", "fx", "interleave"} {
		t.Run(drift, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"process-priced":3}`, "USDExchangeRate": "7"}))
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			var database string
			require.NoError(t, db.Raw("SELECT current_database()").Scan(&database).Error)
			u, err := url.Parse(os.Getenv("TEST_POSTGRES_DSN"))
			require.NoError(t, err)
			u.Path = "/" + database
			require.NoError(t, catalogPostgresDSN(u.String()))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCatalogPublicationProcesses$", "-test.v")
			command.Env = append(os.Environ(), processKey+"=child", "TEST_POSTGRES_DSN="+u.String(), "CCG_CATALOG_PUBLICATION_DRIFT="+drift)
			stdout, err := command.StdoutPipe()
			require.NoError(t, err)
			stdin, err := command.StdinPipe()
			require.NoError(t, err)
			var stderr bytes.Buffer
			command.Stderr = &stderr
			require.NoError(t, command.Start())
			finished := false
			defer func() {
				if !finished {
					cancel()
					_ = command.Wait()
				}
			}()
			reader := bufio.NewReader(stdout)
			await := func(want string) {
				t.Helper()
				for {
					line, readErr := reader.ReadString('\n')
					require.NoError(t, readErr, "child did not reach %s", want)
					if strings.TrimSpace(line) == "catalog-process:"+want {
						return
					}
				}
			}
			await("ready")
			if drift == "interleave" {
				// Admit a request before the independent writer commits, then
				// capture its money after that writer ACKs the new shared epoch.
				// The admitted request must stay wholly on its frozen old input.
				require.NoError(t, WithCatalogPricingRead(context.Background(), "process-priced", func() error {
					_, err := io.WriteString(stdin, "refresh\n")
					require.NoError(t, err)
					await("captured")
					_, err = io.WriteString(stdin, "release\n")
					require.NoError(t, err)
					await("done")
					money, _, err := ResolvePublishedCatalogMoneyContext("process-priced")
					require.NoError(t, err)
					assert.Equal(t, float64(7), money.CNYPerUSD)
					assert.Equal(t, float64(3), ratio_setting.GetModelPriceMap()["process-priced"])
					return nil
				}))
			} else {
				_, err = io.WriteString(stdin, "refresh\n")
				require.NoError(t, err)
				await("captured")
				var during CatalogSyncState
				require.NoError(t, db.First(&during, CatalogSyncStateID).Error)
				if drift == "noop" {
					assert.Equal(t, before, during, "another process's unchanged refresh must not write a shared pending gate")
					assert.NoError(t, WithCatalogPricingRead(context.Background(), "process-priced", func() error {
						assert.Equal(t, float64(3), ratio_setting.GetModelPriceMap()["process-priced"])
						return nil
					}), "the incumbent process must remain able to select its price")
				} else {
					assert.Equal(t, "committed_pending_publish", during.PublicationState)
					assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "process-priced", func() error {
						t.Error("another process's committed FX drift escaped the pending gate")
						return nil
					}), ErrCatalogPublicationPending)
				}
				_, err = io.WriteString(stdin, "release\n")
				require.NoError(t, err)
				await("done")
			}
			_, _ = io.Copy(io.Discard, reader)
			err = command.Wait()
			finished = true
			require.NoError(t, err, stderr.String())
			if drift != "noop" {
				var after CatalogSyncState
				require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
				assert.Equal(t, before.Revision, after.Revision)
				require.Equal(t, "ready", after.PublicationState)
				assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "process-priced", func() error {
					t.Error("the incumbent's old FX epoch escaped after another process acknowledged new FX at the same revision")
					return nil
				}), ErrCatalogPublicationPending)
				require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				require.NoError(t, WithCatalogPricingRead(context.Background(), "process-priced", func() error {
					money, _, err := ResolvePublishedCatalogMoneyContext("process-priced")
					require.NoError(t, err)
					assert.Equal(t, float64(14), money.CNYPerUSD)
					assert.Equal(t, float64(3), ratio_setting.GetModelPriceMap()["process-priced"])
					return nil
				}))
			}
		})
	}
}
