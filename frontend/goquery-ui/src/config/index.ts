export interface FeatureFlags {
  GRAPH_ENABLED: boolean
  MAX_GRAPH_EDGES: number
}

export const featureFlags: FeatureFlags = {
  GRAPH_ENABLED: true,
  MAX_GRAPH_EDGES: 25,
}
