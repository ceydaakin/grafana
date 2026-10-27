package resource

import (
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/apimachinery/utils"
	"github.com/grafana/grafana/pkg/storage/unified/resourcepb"
)

func TestKVStorageBackendSQLKVDoesNotMirrorLegacyTables(t *testing.T) {
	for _, operation := range []string{"write events", "bulk import"} {
		t.Run(operation, func(t *testing.T) {
			store, db := setupSqlKVWithDB(t)
			backend := setupTestStorageBackend(t, withKV(store), func(opts *KVBackendOptions) {
				opts.DisableStorageServices = true
			})
			ctx := t.Context()
			assertNoLegacyWrites := func() {
				t.Helper()
				for _, table := range []string{"resource", "resource_version"} {
					var count int
					require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count))
					require.Zero(t, count, table)
				}
				var versions, previousVersions, generation int64
				require.NoError(t, db.QueryRowContext(ctx,
					"SELECT COUNT(resource_version), COUNT(previous_resource_version), MAX(generation) FROM resource_history",
				).Scan(&versions, &previousVersions, &generation))
				require.Zero(t, versions)
				require.Zero(t, previousVersions)
				require.Zero(t, generation)
			}

			if operation == "bulk import" {
				resp := backend.ProcessBulk(ctx, BulkSettings{
					Collection: []*resourcepb.ResourceKey{{
						Namespace: "test-namespace", Group: testBulkImportGroup, Resource: testBulkImportResource,
					}},
				}, newBatchOnlyBulkIterator([]*resourcepb.BulkRequest{
					newBulkImportRequest("test-namespace", "item-1", resourcepb.BulkRequest_ADDED),
					newBulkImportRequest("test-namespace", "item-1", resourcepb.BulkRequest_MODIFIED),
				}))
				require.Nil(t, resp.Error)
				require.Empty(t, resp.Rejected)
				require.EqualValues(t, 2, resp.Processed)
				assertNoLegacyWrites()
				return
			}

			now := time.Now()
			backend.resourceVersions = newResourceVersionGenerator(42, func() time.Time { return now })
			obj, err := createTestObjectWithName("test-resource", appsNamespace, "value")
			require.NoError(t, err)
			meta, err := utils.MetaAccessor(obj)
			require.NoError(t, err)
			meta.SetGeneration(7)
			var previousRV int64
			for _, eventType := range []resourcepb.WatchEvent_Type{
				resourcepb.WatchEvent_ADDED, resourcepb.WatchEvent_MODIFIED, resourcepb.WatchEvent_DELETED,
			} {
				rv, err := backend.WriteEvent(ctx, WriteEvent{
					Type: eventType, Key: appsKey("test-resource"), Value: objectToJSONBytes(t, obj),
					Object: meta, ObjectOld: meta, PreviousRV: previousRV,
				})
				require.NoError(t, err)
				require.Equal(t, now.UnixMilli(), snowflake.ID(rv).Time())
				require.EqualValues(t, 42, snowflake.ID(rv).Node())
				require.Greater(t, rv, previousRV)
				assertNoLegacyWrites()
				previousRV = rv
				now = now.Add(time.Millisecond)
			}
		})
	}
}
