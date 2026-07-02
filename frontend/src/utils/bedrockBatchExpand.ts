import type { CreateAccountRequest } from '@/types'

export type BedrockProfile = 'geo' | 'global'

export interface BedrockBatchConfig {
  /** Single account-wide Bedrock API key (bearer token). */
  apiKey: string
  /** Region codes to create accounts for, e.g. ['us-east-1', 'eu-west-1']. */
  regions: string[]
  /** Which cross-region profiles to create per region. */
  profiles: BedrockProfile[]
  /** Name prefix; account names are `${prefix}-${region}-${geo|global}`. */
  namePrefix: string
  priority: number
  concurrency: number
  loadFactor: number
  /** When true, sets credentials.pool_mode = true on every account. */
  poolMode: boolean
  /** group_ids assigned to geo-profile accounts. */
  geoGroupIds: number[]
  /** group_ids assigned to global-profile accounts (equals geoGroupIds when not split). */
  globalGroupIds: number[]
}

/**
 * Expand one Bedrock API key into one CreateAccountRequest per (region × profile).
 * Geo vs Global differ ONLY by the aws_force_global credential ('false' vs 'true');
 * the backend derives the region prefix (us./eu./apac./global.) at request time.
 */
export function expandBedrockBatch(config: BedrockBatchConfig): CreateAccountRequest[] {
  const prefix = config.namePrefix.trim() || 'bedrock'
  const apiKey = config.apiKey.trim()
  const out: CreateAccountRequest[] = []

  for (const region of config.regions) {
    for (const profile of config.profiles) {
      const isGlobal = profile === 'global'
      const credentials: Record<string, unknown> = {
        auth_mode: 'apikey',
        api_key: apiKey,
        aws_region: region,
        aws_force_global: isGlobal ? 'true' : 'false',
      }
      if (config.poolMode) credentials.pool_mode = true

      out.push({
        name: `${prefix}-${region}-${isGlobal ? 'global' : 'geo'}`,
        platform: 'anthropic',
        type: 'bedrock',
        credentials,
        extra: {},
        group_ids: isGlobal ? config.globalGroupIds : config.geoGroupIds,
        priority: config.priority,
        concurrency: config.concurrency,
        load_factor: config.loadFactor,
      })
    }
  }

  return out
}
