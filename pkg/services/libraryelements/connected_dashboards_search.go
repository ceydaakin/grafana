package libraryelements

import (
	"context"
	"fmt"

	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	dashboardv0alpha1 "github.com/grafana/grafana/apps/dashboard/pkg/apis/dashboard/v0alpha1"
	"github.com/grafana/grafana/pkg/services/dashboards"
	dashboardsearch "github.com/grafana/grafana/pkg/services/dashboards/service/search"
	"github.com/grafana/grafana/pkg/storage/unified/resourcepb"
)

// searchDashboardsLimit bounds the connected-dashboards search, mirroring
// dashboards.DashboardServiceImpl.GetDashboardsByLibraryPanelUID's own limit.
const searchDashboardsLimit = 100000

// SearchConnectedDashboardsFinder implements ConnectedDashboardsFinder via a direct unified storage
// search, for deployments without a local dashboards.DashboardService (e.g. a folder apiserver
// running standalone). Dashboards are migrated to unified storage even where alert rules and
// library panels are not, so this query needs no legacy database routing of its own.
type SearchConnectedDashboardsFinder struct {
	Searcher resourcepb.ResourceIndexClient
}

// GetDashboardsByLibraryPanelUID mirrors
// dashboards.DashboardServiceImpl.GetDashboardsByLibraryPanelUID's search request, querying the
// search index directly instead of through a local DashboardService. The namespace comes from ctx
// (the same apiserver request namespace LegacyDatabaseProvider resolves the routed database from)
// rather than orgID: in a standalone deployment, every tenant's namespace can parse to the same
// OrgID, so orgID alone cannot identify which tenant's dashboards to search.
func (f *SearchConnectedDashboardsFinder) GetDashboardsByLibraryPanelUID(ctx context.Context, libraryPanelUID string, orgID int64) ([]*dashboards.DashboardRef, error) {
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("expecting namespace in context")
	}

	gvr := dashboardv0alpha1.DashboardResourceInfo.GroupVersionResource()
	req := dashboardsearch.LibraryPanelDashboardsRequest(libraryPanelUID, searchDashboardsLimit)
	req.Options.Key = &resourcepb.ResourceKey{
		Namespace: ns,
		Group:     gvr.Group,
		Resource:  gvr.Resource,
	}

	results, err := dashboardsearch.SearchAll(ctx, orgID, req, func(ctx context.Context, _ int64, req *resourcepb.ResourceSearchRequest) (*resourcepb.ResourceSearchResponse, error) {
		return f.Searcher.Search(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return dashboardsearch.DashboardRefsFromResults(results), nil
}

var _ ConnectedDashboardsFinder = (*SearchConnectedDashboardsFinder)(nil)
