export default {
  title: 'Model Plaza',
  description: 'Browse available models and pricing by group',
  loading: 'Loading...',
  empty: 'No groups to display',
  loadFailed: 'Failed to load model plaza',
  noSearchResult: 'No matching models',
  anonymousHint: 'Sign in to see your exclusive groups and personal rates',
  filters: {
    platformLabel: 'Platform',
    groupLabel: 'Group',
    rateLabel: 'Rate',
    modelLabel: 'Model',
    searchPlaceholder: 'Search models',
    all: 'All'
  },
  badges: {
    exclusive: 'Exclusive',
    subscription: 'Subscription'
  },
  detail: {
    noModels: 'No models configured for this group',
    noPricing: 'Pricing not configured',
    peakNote: 'Peak hours {window}: billing rate ×{multiplier}',
    longContextDisabledNote: 'Long-context tier pricing is disabled for this group: requests above the threshold are billed at the base tier; official tiers are for reference only'
  },
  table: {
    model: 'Model',
    input: 'Input',
    output: 'Output',
    cache: 'Cache',
    cacheWrite: 'Write',
    cacheRead: 'Read',
    cacheWriteShort: 'W',
    cacheReadShort: 'R',
    tierHint: 'The whole request is billed at the tier matching its total context (input + cache write + cache read)',
    tierHintMarginal: 'Only the portion above the threshold is billed at this tier; output is unaffected',
    reasoningMultiplierBadge: '{effort} ×{multiplier}',
    reasoningMultiplierHint: 'When the forwarded reasoning effort is {effort}, billing and quota usage for the request are multiplied by {multiplier}. Unconfigured levels use 1×',
    marginalBadge: 'excess-only tiers',
    timePricingRowHint: 'Requests made within this period ({timezone} time) are billed at the prices in this row',
    timePricingRowHintWeekdays:
      'On weekdays (Mon–Fri) only, requests made within this period ({timezone} time) are billed at the prices in this row; weekends use the standard prices',
    timePricingRowHintPeak:
      '; prices in this row exclude the peak-hour rate — where this period overlaps the peak hours {window}, the overlapping portion is additionally multiplied by ×{multiplier}',
    timePricingWeekdays: 'Weekdays',
    timePricingRateHint: 'Effective rate {rate} × period multiplier {multiplier}',
    paidPrice: 'Your Price (Discounted)',
    officialPrice: 'Official Price',
    rate: 'Rate',
    unitPerMillion: '$ / 1M tokens',
    perUnitRequest: '/ request',
    perUnitImage: '/ image',
    perUnitVideo: '/ second',
    perRequest: 'Per request',
    perImage: 'Per image',
    perVideo: 'Per second'
  },
  nav: {
    login: 'Sign In',
    backToDashboard: 'Back to Console'
  }
}
