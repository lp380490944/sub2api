import { describe, expect, it } from 'vitest'
import { expandBedrockBatch, type BedrockBatchConfig } from '@/utils/bedrockBatchExpand'

const base: BedrockBatchConfig = {
  apiKey: '  sk-bedrock-abc  ',
  regions: ['us-east-1', 'eu-west-1'],
  profiles: ['geo', 'global'],
  namePrefix: 'bedrock',
  priority: 50,
  concurrency: 5,
  loadFactor: 1,
  geoGroupIds: [7],
  globalGroupIds: [7],
}

describe('expandBedrockBatch', () => {
  it('produces regions × profiles accounts', () => {
    expect(expandBedrockBatch(base)).toHaveLength(4)
  })

  it('sets apikey auth_mode, trimmed key, region, and aws_force_global per profile', () => {
    const accts = expandBedrockBatch(base)
    const geo = accts.find((a) => a.name === 'bedrock-us-east-1-geo')!
    const glob = accts.find((a) => a.name === 'bedrock-us-east-1-global')!
    expect(geo.platform).toBe('anthropic')
    expect(geo.type).toBe('bedrock')
    expect(geo.credentials).toMatchObject({
      auth_mode: 'apikey',
      api_key: 'sk-bedrock-abc',
      aws_region: 'us-east-1',
      aws_force_global: 'false',
    })
    expect(glob.credentials).toMatchObject({ aws_force_global: 'true' })
  })

  it('applies shared priority/concurrency/load_factor to every account', () => {
    for (const a of expandBedrockBatch(base)) {
      expect(a.priority).toBe(50)
      expect(a.concurrency).toBe(5)
      expect(a.load_factor).toBe(1)
    }
  })

  it('routes geo and global accounts to their respective group ids', () => {
    const split = expandBedrockBatch({ ...base, geoGroupIds: [1], globalGroupIds: [2] })
    expect(split.find((a) => a.name.endsWith('-geo'))!.group_ids).toEqual([1])
    expect(split.find((a) => a.name.endsWith('-global'))!.group_ids).toEqual([2])
  })

  it('falls back to a default name prefix when blank', () => {
    const accts = expandBedrockBatch({ ...base, namePrefix: '   ', regions: ['us-east-1'], profiles: ['geo'] })
    expect(accts[0].name).toBe('bedrock-us-east-1-geo')
  })

  it('returns empty when no regions or no profiles', () => {
    expect(expandBedrockBatch({ ...base, regions: [] })).toHaveLength(0)
    expect(expandBedrockBatch({ ...base, profiles: [] })).toHaveLength(0)
  })

  it('skips the Geo account for global-only regions (no geographic profile)', () => {
    // sa-east-1 has no geo profile → only a global account; us-east-1 (geo-capable) gets both.
    const accts = expandBedrockBatch({ ...base, regions: ['us-east-1', 'sa-east-1'], profiles: ['geo', 'global'] })
    const names = accts.map((a) => a.name).sort()
    expect(names).toEqual(['bedrock-sa-east-1-global', 'bedrock-us-east-1-geo', 'bedrock-us-east-1-global'])
    expect(names).not.toContain('bedrock-sa-east-1-geo')
  })

  it('creates no accounts when only Geo is selected for global-only regions', () => {
    expect(expandBedrockBatch({ ...base, regions: ['sa-east-1', 'me-central-1'], profiles: ['geo'] })).toHaveLength(0)
  })
})
