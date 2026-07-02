import { describe, expect, it } from 'vitest'
import {
  AWS_BEDROCK_REGIONS,
  awsBedrockGeoFamily,
  awsBedrockRegionHasGeo,
  commercialBedrockRegionCodes,
} from '@/constants/account'

describe('AWS_BEDROCK_REGIONS', () => {
  it('exposes exactly 34 commercial region codes', () => {
    expect(commercialBedrockRegionCodes()).toHaveLength(34)
  })

  it('has unique region codes across all groups', () => {
    const codes = AWS_BEDROCK_REGIONS.flatMap((g) => g.options.map((o) => o.code))
    expect(new Set(codes).size).toBe(codes.length)
  })

  it('includes new regions missing from the old dropdown', () => {
    const commercial = commercialBedrockRegionCodes()
    for (const code of ['ap-east-1', 'ap-east-2', 'mx-central-1', 'me-central-1', 'il-central-1', 'ca-west-1']) {
      expect(commercial).toContain(code)
    }
  })

  it('excludes GovCloud from the commercial (default-selected) set', () => {
    const commercial = commercialBedrockRegionCodes()
    expect(commercial).not.toContain('us-gov-east-1')
    expect(commercial).not.toContain('us-gov-west-1')
  })
})

describe('awsBedrockGeoFamily', () => {
  it('maps geo-capable regions to their profile family (per Anthropic current-model table)', () => {
    expect(awsBedrockGeoFamily('us-east-1')).toBe('us')
    expect(awsBedrockGeoFamily('us-west-1')).toBe('us')
    expect(awsBedrockGeoFamily('ca-central-1')).toBe('us') // Canada Central is US geo
    expect(awsBedrockGeoFamily('us-gov-east-1')).toBe('us-gov')
    expect(awsBedrockGeoFamily('eu-west-3')).toBe('eu')
    expect(awsBedrockGeoFamily('ap-northeast-1')).toBe('jp')
    expect(awsBedrockGeoFamily('ap-northeast-3')).toBe('jp')
    expect(awsBedrockGeoFamily('ap-southeast-2')).toBe('au')
    expect(awsBedrockGeoFamily('ap-southeast-4')).toBe('au')
  })

  it('returns "" for regions with no geographic profile (global-only)', () => {
    for (const code of [
      'sa-east-1', 'ca-west-1', 'af-south-1', 'me-central-1', 'me-south-1',
      'il-central-1', 'mx-central-1', 'ap-south-1', 'ap-south-2',
      'ap-southeast-1', 'ap-southeast-3', 'ap-northeast-2', 'ap-east-1', 'ap-east-2',
    ]) {
      expect(awsBedrockGeoFamily(code)).toBe('')
      expect(awsBedrockRegionHasGeo(code)).toBe(false)
    }
  })

  it('awsBedrockRegionHasGeo is true only for geo-capable regions', () => {
    expect(awsBedrockRegionHasGeo('us-east-1')).toBe(true)
    expect(awsBedrockRegionHasGeo('ca-central-1')).toBe(true)
    expect(awsBedrockRegionHasGeo('eu-west-1')).toBe(true)
    expect(awsBedrockRegionHasGeo('ap-northeast-1')).toBe(true)
    expect(awsBedrockRegionHasGeo('sa-east-1')).toBe(false)
  })
})
