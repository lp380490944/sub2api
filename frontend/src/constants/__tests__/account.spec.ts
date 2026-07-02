import { describe, expect, it } from 'vitest'
import {
  AWS_BEDROCK_REGIONS,
  awsBedrockGeoFamily,
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
  it('maps regions to the correct geo-profile family', () => {
    expect(awsBedrockGeoFamily('us-east-1')).toBe('us')
    expect(awsBedrockGeoFamily('us-gov-east-1')).toBe('us-gov')
    expect(awsBedrockGeoFamily('eu-west-3')).toBe('eu')
    expect(awsBedrockGeoFamily('ap-northeast-1')).toBe('jp')
    expect(awsBedrockGeoFamily('ap-southeast-2')).toBe('au')
    expect(awsBedrockGeoFamily('ap-south-1')).toBe('apac')
    expect(awsBedrockGeoFamily('sa-east-1')).toBe('us')
    expect(awsBedrockGeoFamily('me-central-1')).toBe('us')
  })
})
