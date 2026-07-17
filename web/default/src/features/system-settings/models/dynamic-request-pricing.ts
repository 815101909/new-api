const dynamicViduModels = new Set([
  'viduq3-pro',
  'viduq3-turbo',
  'viduq3-pro-fast',
  'viduq2-pro-fast',
  'viduq2-pro',
  'viduq2-turbo',
  'viduq2',
  'viduq1',
  'viduq1-classic',
  'vidu2.0',
])

const normalizeModelName = (modelName?: string) =>
  modelName?.trim().toLowerCase() || ''

export const isDynamicRequestPricingModel = (modelName?: string) =>
  dynamicViduModels.has(normalizeModelName(modelName))

export const getDynamicRequestPricingProfileKey = (modelName?: string) => {
  const normalized = normalizeModelName(modelName)
  if (!isDynamicRequestPricingModel(normalized)) return ''
  if (normalized === 'vidu2.0') return 'Default 4s 360p request'
  if (normalized === 'viduq1' || normalized === 'viduq1-classic') {
    return 'Default 5s 1080p request'
  }
  return 'Default 5s 720p request'
}
