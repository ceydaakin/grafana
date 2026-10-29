import { mapValues, omit } from 'lodash';

import { type Dashboard, type Panel, type RowPanel } from '@grafana/schema';
import { type Spec as DashboardV2Spec } from '@grafana/schema/apis/dashboard.grafana.app/v2';

import { type Diffs, jsonDiff } from '../settings/version-history/utils';

/**
 * Diffs two dashboard save models without counting panel plugin versions.
 * A panel only bumps its plugin version when it renders for the first time, so a panel that
 * had not rendered before the last save would otherwise always show up as a change.
 */
export function getDashboardDiff(initial: DashboardV2Spec | Dashboard, changed: DashboardV2Spec | Dashboard): Diffs {
  return jsonDiff(withoutPluginVersions(initial), withoutPluginVersions(changed));
}

function withoutPluginVersions(model: DashboardV2Spec | Dashboard): DashboardV2Spec | Dashboard {
  if ('elements' in model) {
    return {
      ...model,
      elements: mapValues(model.elements, (element) =>
        element.kind === 'Panel'
          ? { ...element, spec: { ...element.spec, vizConfig: { ...element.spec.vizConfig, version: '' } } }
          : element
      ),
    };
  }

  if (!model.panels) {
    return model;
  }

  return { ...model, panels: model.panels.map(withoutPanelPluginVersion) };
}

function withoutPanelPluginVersion(panel: Panel | RowPanel): Panel | RowPanel {
  if (panel.type === 'row' && 'panels' in panel) {
    return { ...panel, panels: panel.panels.map(withoutPanelPluginVersion) };
  }
  return omit(panel, 'pluginVersion');
}
