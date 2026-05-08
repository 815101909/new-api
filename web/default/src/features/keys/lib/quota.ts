import { getCurrencyDisplay } from '@/lib/currency'

export function formatApiKeyQuotaPrecise(quota: number): string {
  const { config, meta } = getCurrencyDisplay()
  const amount = Number(quota || 0)

  if (meta.kind === 'tokens') {
    return Math.round(amount).toLocaleString()
  }

  const quotaPerUnit = Number(config.quotaPerUnit || 0)
  const displayAmount = quotaPerUnit > 0 ? amount / quotaPerUnit : 0

  if (meta.kind === 'currency') {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: meta.currencyCode,
      currencyDisplay: 'narrowSymbol',
      minimumFractionDigits: 6,
      maximumFractionDigits: 6,
    }).format(displayAmount)
  }

  return `${meta.symbol}${displayAmount.toFixed(6)}`
}

export function parseApiKeyQuotaAmount(amount: number): number {
  if (!Number.isFinite(amount)) return 0

  const { config, meta } = getCurrencyDisplay()

  if (meta.kind === 'tokens') {
    return Math.round(amount)
  }

  return Math.round(amount * Number(config.quotaPerUnit || 0))
}

export function quotaUnitsToApiKeyAmount(quota: number): number {
  const { config, meta } = getCurrencyDisplay()

  if (meta.kind === 'tokens') {
    return Number(quota || 0)
  }

  const quotaPerUnit = Number(config.quotaPerUnit || 0)
  return quotaPerUnit > 0
    ? Number((Number(quota || 0) / quotaPerUnit).toFixed(6))
    : 0
}
