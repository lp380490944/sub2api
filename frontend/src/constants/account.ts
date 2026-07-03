/** Types that support account-level quota / pool mode (align with domain.SupportsAccountQuotaType). */
export function isAccountQuotaEligibleType(type: string | undefined | null): boolean {
  if (!type) return false
  return type === 'apikey' || type === 'bedrock' || type === 'vertex' || type === 'service_account'
}

/**
 * Whether the account can carry cache-policy settings (TTL override, user-level
 * cache strategy preference, etc). Must mirror Account.SupportsCachePolicy() in
 * backend/internal/service/account.go — both lists must move together to avoid
 * "UI shows it but backend ignores it" or vice versa.
 */
export function supportsCachePolicy(
  platform: string | undefined | null,
  type: string | undefined | null
): boolean {
  if (platform !== 'anthropic') return false
  switch (type) {
    case 'oauth':
    case 'setup-token':
    case 'apikey':
    case 'bedrock':
    case 'service_account':
    case 'vertex':
      return true
    default:
      return false
  }
}

/** WebSearch emulation mode values (must match backend WebSearchMode* constants in account.go) */
export const WEB_SEARCH_MODE_DEFAULT = 'default' as const
export const WEB_SEARCH_MODE_ENABLED = 'enabled' as const
export const WEB_SEARCH_MODE_DISABLED = 'disabled' as const
export type WebSearchMode = typeof WEB_SEARCH_MODE_DEFAULT | typeof WEB_SEARCH_MODE_ENABLED | typeof WEB_SEARCH_MODE_DISABLED

/** Quota notification threshold type values (must match thresholdType* constants in balance_notify_service.go) */
export const QUOTA_THRESHOLD_TYPE_FIXED = 'fixed' as const
export const QUOTA_THRESHOLD_TYPE_PERCENTAGE = 'percentage' as const
export type QuotaThresholdType = typeof QUOTA_THRESHOLD_TYPE_FIXED | typeof QUOTA_THRESHOLD_TYPE_PERCENTAGE

/** Quota reset mode values */
export const QUOTA_RESET_MODE_ROLLING = 'rolling' as const
export const QUOTA_RESET_MODE_FIXED = 'fixed' as const
export type QuotaResetMode = typeof QUOTA_RESET_MODE_ROLLING | typeof QUOTA_RESET_MODE_FIXED

/** Vertex AI location options for Service Account accounts */
export const VERTEX_LOCATION_OPTIONS = [
  {
    label: 'Common',
    options: [
      { value: 'us-central1', label: 'us-central1 (Iowa)' },
      { value: 'global', label: 'global' },
      { value: 'us', label: 'us' },
      { value: 'eu', label: 'eu' }
    ]
  },
  {
    label: 'United States',
    options: [
      { value: 'us-east1', label: 'us-east1 (South Carolina)' },
      { value: 'us-east4', label: 'us-east4 (Northern Virginia)' },
      { value: 'us-east5', label: 'us-east5 (Columbus)' },
      { value: 'us-south1', label: 'us-south1 (Dallas)' },
      { value: 'us-west1', label: 'us-west1 (Oregon)' },
      { value: 'us-west4', label: 'us-west4 (Las Vegas)' }
    ]
  },
  {
    label: 'Europe',
    options: [
      { value: 'europe-west1', label: 'europe-west1 (Belgium)' },
      { value: 'europe-west2', label: 'europe-west2 (London)' },
      { value: 'europe-west3', label: 'europe-west3 (Frankfurt)' },
      { value: 'europe-west4', label: 'europe-west4 (Netherlands)' },
      { value: 'europe-west6', label: 'europe-west6 (Zurich)' },
      { value: 'europe-west8', label: 'europe-west8 (Milan)' },
      { value: 'europe-west9', label: 'europe-west9 (Paris)' }
    ]
  },
  {
    label: 'Asia Pacific',
    options: [
      { value: 'asia-east1', label: 'asia-east1 (Taiwan)' },
      { value: 'asia-east2', label: 'asia-east2 (Hong Kong)' },
      { value: 'asia-northeast1', label: 'asia-northeast1 (Tokyo)' },
      { value: 'asia-northeast3', label: 'asia-northeast3 (Seoul)' },
      { value: 'asia-south1', label: 'asia-south1 (Mumbai)' },
      { value: 'asia-southeast1', label: 'asia-southeast1 (Singapore)' },
      { value: 'australia-southeast1', label: 'australia-southeast1 (Sydney)' }
    ]
  }
] as const

/** One selectable AWS region for Bedrock. */
export interface AwsBedrockRegion {
  code: string
  city: string
}

/** A geography-grouped block of Bedrock regions. `commercial:false` = not selected by default (GovCloud). */
export interface AwsBedrockRegionGroup {
  label: string
  commercial: boolean
  options: AwsBedrockRegion[]
}

/**
 * Canonical AWS region list for Bedrock accounts. Single source of truth shared by the
 * single-create region <select> and the batch-import board. Mirrors the AWS console region
 * picker (commercial regions); GovCloud kept but flagged non-commercial (not default-selected).
 */
export const AWS_BEDROCK_REGIONS: AwsBedrockRegionGroup[] = [
  {
    label: 'US',
    commercial: true,
    options: [
      { code: 'us-east-1', city: 'N. Virginia' },
      { code: 'us-east-2', city: 'Ohio' },
      { code: 'us-west-1', city: 'N. California' },
      { code: 'us-west-2', city: 'Oregon' },
    ],
  },
  {
    label: 'Africa',
    commercial: true,
    options: [{ code: 'af-south-1', city: 'Cape Town' }],
  },
  {
    label: 'Asia Pacific',
    commercial: true,
    options: [
      { code: 'ap-east-1', city: 'Hong Kong' },
      { code: 'ap-east-2', city: 'Taipei' },
      { code: 'ap-south-1', city: 'Mumbai' },
      { code: 'ap-south-2', city: 'Hyderabad' },
      { code: 'ap-southeast-1', city: 'Singapore' },
      { code: 'ap-southeast-2', city: 'Sydney' },
      { code: 'ap-southeast-3', city: 'Jakarta' },
      { code: 'ap-southeast-4', city: 'Melbourne' },
      { code: 'ap-southeast-5', city: 'Malaysia' },
      { code: 'ap-southeast-6', city: 'New Zealand' },
      { code: 'ap-southeast-7', city: 'Thailand' },
      { code: 'ap-northeast-1', city: 'Tokyo' },
      { code: 'ap-northeast-2', city: 'Seoul' },
      { code: 'ap-northeast-3', city: 'Osaka' },
    ],
  },
  {
    label: 'Canada',
    commercial: true,
    options: [
      { code: 'ca-central-1', city: 'Central' },
      { code: 'ca-west-1', city: 'Calgary' },
    ],
  },
  {
    label: 'Europe',
    commercial: true,
    options: [
      { code: 'eu-central-1', city: 'Frankfurt' },
      { code: 'eu-central-2', city: 'Zurich' },
      { code: 'eu-west-1', city: 'Ireland' },
      { code: 'eu-west-2', city: 'London' },
      { code: 'eu-west-3', city: 'Paris' },
      { code: 'eu-south-1', city: 'Milan' },
      { code: 'eu-south-2', city: 'Spain' },
      { code: 'eu-north-1', city: 'Stockholm' },
    ],
  },
  {
    label: 'Mexico',
    commercial: true,
    options: [{ code: 'mx-central-1', city: 'Central' }],
  },
  {
    label: 'Middle East',
    commercial: true,
    options: [
      { code: 'me-south-1', city: 'Bahrain' },
      { code: 'me-central-1', city: 'UAE' },
    ],
  },
  {
    label: 'Israel',
    commercial: true,
    options: [{ code: 'il-central-1', city: 'Tel Aviv' }],
  },
  {
    label: 'South America',
    commercial: true,
    options: [{ code: 'sa-east-1', city: 'São Paulo' }],
  },
  {
    label: 'GovCloud',
    commercial: false,
    options: [
      { code: 'us-gov-east-1', city: 'GovCloud US-East' },
      { code: 'us-gov-west-1', city: 'GovCloud US-West' },
    ],
  },
]

/** Flat list of commercial region codes (default selection for the batch board). */
export function commercialBedrockRegionCodes(): string[] {
  return AWS_BEDROCK_REGIONS.filter((g) => g.commercial).flatMap((g) => g.options.map((o) => o.code))
}

/**
 * Geographic cross-region inference family for a region, per Anthropic's current-model
 * Bedrock region table (US / EU / JP / AU + GovCloud). Returns '' when the region has NO
 * geographic profile — those regions are reachable only via the global. profile, so a
 * "Geo" account there is invalid (e.g. sa-east-1 + us. → 400 invalid model identifier).
 * Mirrors backend BedrockCrossRegionPrefix (bedrock_request.go), except the backend
 * returns 'global' for the no-geo case while the UI uses '' to mean "no geo, global only".
 * Note: there is no apac. profile for current Claude models (Claude-3 era only).
 */
export function awsBedrockGeoFamily(region: string): string {
  if (region.startsWith('us-gov')) return 'us-gov'
  if (region === 'ca-central-1' || region.startsWith('us-')) return 'us'
  if (region.startsWith('eu-')) return 'eu'
  if (region === 'ap-northeast-1' || region === 'ap-northeast-3') return 'jp'
  if (region === 'ap-southeast-2' || region === 'ap-southeast-4') return 'au'
  return '' // no geographic profile → global-only
}

/** Whether a region has a geographic cross-region inference profile (can get a "Geo" account). */
export function awsBedrockRegionHasGeo(region: string): boolean {
  return awsBedrockGeoFamily(region) !== ''
}
