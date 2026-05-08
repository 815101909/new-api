/**
 * ============================================================================
 * Currency Formatting Library
 * ============================================================================
 *
 * This module provides currency formatting utilities for RMB-based quota
 * amounts and token displays based on admin-configured settings.
 *
 * ## Key Concepts
 *
 * 1. **System CNY**: Internal currency unit used throughout the system (e.g., ¥10)
 * 2. **Display Currency**: Admin-configured display currency (e.g., CNY, custom currency)
 * 3. **Recharge Price (priceRatio)**: Cost in payment currency to purchase ¥1 credit
 * 4. **Tokens**: Alternative display unit (e.g., 500,000 quota units = ¥1)
 *
 * ## When to Use Each Function
 *
 * - `formatCurrencyFromCNY()`: Formats system CNY amounts
 * - `formatBillingCurrencyFromCNY()`: Use for billing/pricing displays (never shows tokens)
 * - `formatLocalCurrencyAmount()`: Use for payment amounts already in local currency
 * - `formatQuotaWithCurrency()`: Use for raw quota values (converts to CNY first)
 *
 * ## Example Scenario
 *
 * Admin Configuration:
 * - quotaDisplayType: 'CNY'
 * - priceRatio: 1 (¥1 payment per ¥1 credit)
 * - quotaPerUnit: 500000 (quota units per CNY)
 *
 * User Flow:
 * 1. Recharge option: ¥10
 *    - Display: formatCurrencyFromCNY(10) → "¥10"
 * 2. Payment amount: 10 × 1 = 10 (already in CNY)
 *    - Display: formatLocalCurrencyAmount(50) → "¥50"
 * 3. User receives: ¥10 credit
 *    - Balance display: formatCurrencyFromCNY(10) → "¥10"
 *
 * ## Quick Reference Guide
 *
 * | Scenario | Input Type | Function to Use | Why |
 * |----------|-----------|-----------------|-----|
 * | User balance display | CNY (from DB) | `formatCurrencyFromCNY()` | Formats stored amount |
 * | Recharge option button | CNY | `formatCurrencyFromCNY()` | Formats stored amount |
 * | Payment confirmation | Already local currency | `formatLocalCurrencyAmount()` | Already converted via priceRatio |
 * | Billing history Amount | CNY (from DB) | `formatCurrencyFromCNY()` | Formats stored amount |
 * | Billing history Payment | Local currency | `formatNumber()` | Just show number, no symbol |
 * | Model pricing | CNY | `formatBillingCurrencyFromCNY()` | Never show as tokens |
 * | Raw quota from API | Tokens | `formatQuotaWithCurrency()` | Convert quota → CNY → display |
 *
 * ## Critical Rules
 *
 * 1. **Never double-convert**: If you multiply by exchangeRate, use formatLocalCurrencyAmount()
 * 2. **Database CNY values**: Always use formatCurrencyFromCNY() for stored quota amounts
 * 3. **Payment amounts**: Always use formatLocalCurrencyAmount() for priceRatio-calculated values
 * 4. **Billing displays**: Use formatBillingCurrencyFromCNY() to avoid token display
 * 5. **No exchange-rate base**: RMB is the base amount.
 */
import {
  useSystemConfigStore,
  DEFAULT_CURRENCY_CONFIG,
  type CurrencyConfig,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

export interface CurrencyFormatOptions {
  /** Fraction digits to use when |value| >= 1 */
  digitsLarge?: number
  /** Fraction digits to use when |value| < 1 */
  digitsSmall?: number
  /** Whether to abbreviate thousands with k suffix */
  abbreviate?: boolean
  /** Minimal absolute value to display when rounding would produce zero */
  minimumNonZero?: number
}

type DisplayMeta =
  | {
      kind: 'currency'
      symbol: string
      currencyCode: string
      exchangeRate: number
    }
  | {
      kind: 'custom'
      symbol: string
      exchangeRate: number
    }
  | {
      kind: 'tokens'
      /** Number of quota units per CNY */
      quotaPerUnit: number
    }

const DEFAULT_FORMAT_OPTIONS: Required<CurrencyFormatOptions> = {
  digitsLarge: 2,
  digitsSmall: 4,
  abbreviate: true,
  minimumNonZero: 0,
}

const DISPLAY_TYPE_VALUES = ['CNY', 'TOKENS', 'CUSTOM'] as const
type DisplayTypeLiteral = (typeof DISPLAY_TYPE_VALUES)[number]

export function isCurrencyDisplayType(
  value: unknown
): value is CurrencyDisplayType {
  return (
    typeof value === 'string' &&
    DISPLAY_TYPE_VALUES.includes(value as DisplayTypeLiteral)
  )
}

export function parseCurrencyDisplayType(
  value: unknown,
  fallback: CurrencyDisplayType = 'CNY'
): CurrencyDisplayType {
  return isCurrencyDisplayType(value) ? value : fallback
}

function getConfig(): CurrencyConfig {
  const { config } = useSystemConfigStore.getState()
  const currency = config?.currency ?? DEFAULT_CURRENCY_CONFIG
  return {
    ...DEFAULT_CURRENCY_CONFIG,
    ...currency,
    quotaPerUnit:
      currency?.quotaPerUnit && currency.quotaPerUnit > 0
        ? currency.quotaPerUnit
        : DEFAULT_CURRENCY_CONFIG.quotaPerUnit,
    customCurrencyExchangeRate:
      currency?.customCurrencyExchangeRate &&
      currency.customCurrencyExchangeRate > 0
        ? currency.customCurrencyExchangeRate
        : DEFAULT_CURRENCY_CONFIG.customCurrencyExchangeRate,
    customCurrencySymbol:
      currency?.customCurrencySymbol?.trim() ||
      DEFAULT_CURRENCY_CONFIG.customCurrencySymbol,
  }
}

function getDisplayMeta(config: CurrencyConfig): DisplayMeta {
  switch (config.quotaDisplayType) {
    case 'CNY':
      return {
        kind: 'currency',
        symbol: '¥',
        currencyCode: 'CNY',
        exchangeRate: 1,
      }
    case 'CUSTOM':
      return {
        kind: 'custom',
        symbol: config.customCurrencySymbol,
        exchangeRate: config.customCurrencyExchangeRate,
      }
    case 'TOKENS':
      return {
        kind: 'tokens',
        quotaPerUnit: config.quotaPerUnit,
      }
    default:
      return {
        kind: 'currency',
        symbol: '¥',
        currencyCode: 'CNY',
        exchangeRate: 1,
      }
  }
}

function getBillingDisplayMeta(config: CurrencyConfig): DisplayMeta {
  const meta = getDisplayMeta(config)
  if (meta.kind === 'tokens') {
    return {
      kind: 'currency',
      symbol: '¥',
      currencyCode: 'CNY',
      exchangeRate: 1,
    }
  }
  return meta
}

function mergeOptions(
  options?: CurrencyFormatOptions
): Required<CurrencyFormatOptions> {
  if (!options) return DEFAULT_FORMAT_OPTIONS
  return {
    digitsLarge: options.digitsLarge ?? DEFAULT_FORMAT_OPTIONS.digitsLarge,
    digitsSmall: options.digitsSmall ?? DEFAULT_FORMAT_OPTIONS.digitsSmall,
    abbreviate: options.abbreviate ?? DEFAULT_FORMAT_OPTIONS.abbreviate,
    minimumNonZero:
      options.minimumNonZero ?? DEFAULT_FORMAT_OPTIONS.minimumNonZero,
  }
}

function removeTrailingZeros(str: string): string {
  if (!str.includes('.')) return str
  return str.replace(/(\.[0-9]*?)0+$/, '$1').replace(/\.$/, '')
}

function formatNumberWithSuffix(
  value: number,
  digitsLarge: number,
  digitsSmall: number,
  abbreviate: boolean
): string {
  const abs = Math.abs(value)
  if (abbreviate && abs >= 1000) {
    const result = value / 1000
    return removeTrailingZeros(result.toFixed(1)) + 'k'
  }

  const digits = abs >= 1 ? digitsLarge : digitsSmall
  return removeTrailingZeros(value.toFixed(digits))
}

function adjustForMinimum(
  value: number,
  digits: number,
  minimumNonZero: number
): number {
  if (value === 0) return value

  const threshold = minimumNonZero > 0 ? minimumNonZero : Math.pow(10, -digits)
  const abs = Math.abs(value)
  if (abs > 0 && abs < threshold) {
    return value > 0 ? threshold : -threshold
  }
  return value
}

function formatCurrencyValue(
  value: number,
  options: Required<CurrencyFormatOptions>,
  meta: DisplayMeta
): string {
  if (meta.kind === 'tokens') {
    return formatNumberWithSuffix(
      value,
      options.digitsLarge,
      options.digitsSmall,
      options.abbreviate
    )
  }

  const digits =
    Math.abs(value) >= 1 ? options.digitsLarge : options.digitsSmall
  const adjustedValue = adjustForMinimum(value, digits, options.minimumNonZero)

  if (meta.kind === 'currency') {
    const formatted = new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: meta.currencyCode,
      currencyDisplay: 'narrowSymbol',
      minimumFractionDigits: 0,
      maximumFractionDigits: digits,
    }).format(adjustedValue)
    return formatted
  }

  const decimal = new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: digits,
  }).format(adjustedValue)

  return `${meta.symbol}${decimal}`
}

/**
 * Get the current currency configuration and display metadata.
 *
 * @returns Object containing config and display metadata
 *
 * @internal
 * This is primarily for internal use. Most consumers should use the
 * higher-level formatting functions instead.
 */
export function getCurrencyDisplay() {
  const config = getConfig()
  const meta = getDisplayMeta(config)
  return { config, meta }
}

/**
 * Format a system CNY amount according to the admin-configured display settings.
 *
 * This is the PRIMARY function for displaying quota/balance/credit amounts
 * that are stored in the system as CNY values.
 *
 * @param amountCNY - Amount in system CNY units
 * @param options - Optional formatting configuration
 * @returns Formatted string with currency symbol or token count
 *
 * @example
 * // With quotaDisplayType: 'CNY'
 * formatCurrencyFromCNY(10) → "¥10"
 *
 * @example
 * // With quotaDisplayType: 'TOKENS', quotaPerUnit: 500000
 * formatCurrencyFromCNY(10) → "5,000,000"
 *
 * @example
 * // With quotaDisplayType: 'CUSTOM', customCurrencySymbol: '€', customCurrencyExchangeRate: 0.9
 * formatCurrencyFromCNY(10) → "€9"
 *
 * @remarks
 * Use this function for:
 * - User balance/quota display
 * - Recharge option amounts
 * - Transaction amounts in billing history
 * - Any value stored in database as CNY
 *
 * DO NOT use for:
 * - Payment amounts already converted via priceRatio → use formatLocalCurrencyAmount()
 * - Raw token values → use formatQuotaWithCurrency()
 */
export function formatCurrencyFromCNY(
  amountCNY: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (amountCNY == null || Number.isNaN(amountCNY)) return '-'

  const { config, meta } = getCurrencyDisplay()
  const merged = mergeOptions(options)

  if (meta.kind === 'tokens') {
    const tokens = amountCNY * config.quotaPerUnit
    return formatNumberWithSuffix(
      tokens,
      0,
      merged.digitsSmall,
      merged.abbreviate
    )
  }

  const value =
    meta.kind === 'currency'
      ? amountCNY * meta.exchangeRate
      : amountCNY * meta.exchangeRate

  return formatCurrencyValue(value, merged, meta)
}

/**
 * Format RMB amounts for billing/payment contexts (never shows tokens).
 *
 * Similar to formatCurrencyFromCNY, but NEVER displays in token units.
 * Always shows real currency values (CNY or custom currency) even when the system
 * is configured to display quotas as tokens elsewhere.
 *
 * @param amountCNY - Amount in system CNY units
 * @param options - Optional formatting configuration
 * @returns Formatted string with currency symbol (never tokens)
 *
 * @example
 * // With quotaDisplayType: 'TOKENS' - still shows currency
 * formatBillingCurrencyFromCNY(10) → "¥10"  (not "5,000,000 tokens")
 *
 * @remarks
 * Use this function for:
 * - Model pricing displays
 * - API usage costs
 * - Billing/invoice amounts
 * - Any monetary value where tokens don't make sense
 *
 * DO NOT use for:
 * - User balance/quota → use formatCurrencyFromCNY()
 * - Payment amounts already in local currency → use formatLocalCurrencyAmount()
 */
export function formatBillingCurrencyFromCNY(
  amountCNY: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (amountCNY == null || Number.isNaN(amountCNY)) return '-'

  const { config } = getCurrencyDisplay()
  const meta = getBillingDisplayMeta(config)
  const merged = mergeOptions(options)
  const value =
    meta.kind === 'currency' || meta.kind === 'custom'
      ? amountCNY * meta.exchangeRate
      : amountCNY

  return formatCurrencyValue(value, merged, meta)
}

/**
 * Format raw quota values (token units) to display currency.
 *
 * Converts raw quota/token amounts to CNY first, then formats according
 * to display settings. Use when you have quota in token units (e.g., 5000000)
 * and need to display it as currency (e.g., "¥10").
 *
 * @param quota - Raw quota amount in token units (e.g., 5000000)
 * @param options - Optional formatting configuration
 * @returns Formatted string with currency symbol or token count
 *
 * @example
 * // With quotaPerUnit: 500000, quotaDisplayType: 'CNY'
 * formatQuotaWithCurrency(5000000) → "¥10"
 *
 * @remarks
 * Use this function for:
 * - Raw quota values from database (stored as tokens)
 * - When you need to convert quota units → CNY → display currency
 *
 * DO NOT use for:
 * - Values already in CNY → use formatCurrencyFromCNY()
 * - Payment amounts → use formatLocalCurrencyAmount()
 */
export function formatQuotaWithCurrency(
  quota: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (quota == null || Number.isNaN(quota)) return '-'

  const { config } = getCurrencyDisplay()
  const amountCNY = quota / config.quotaPerUnit
  return formatCurrencyFromCNY(amountCNY, options)
}

/**
 * Get the current currency label for UI display.
 *
 * Returns a simple string label representing the current display currency.
 * Useful for labels, tooltips, and UI text.
 *
 * @returns Currency label string (e.g., "CNY", "Tokens")
 *
 * @example
 * getCurrencyLabel() → "CNY"
 * getCurrencyLabel() → "Tokens"
 *
 * @remarks
 * Use this for:
 * - Currency selector labels
 * - Table column headers
 * - Form field labels
 */
export function getCurrencyLabel(): string {
  const { config, meta } = getCurrencyDisplay()

  if (meta.kind === 'tokens') {
    return 'Tokens'
  }

  switch (config.quotaDisplayType) {
    case 'CNY':
      return 'CNY'
    case 'CUSTOM':
      return meta.kind === 'custom' ? meta.symbol : 'Custom'
    default:
      return 'CNY'
  }
}

/**
 * Check if currency display is enabled (not in token-only mode).
 *
 * @returns True if displaying in actual currency, false if tokens only
 *
 * @example
 * // With quotaDisplayType: 'CNY'
 * isCurrencyDisplayEnabled() → true
 *
 * // With quotaDisplayType: 'TOKENS'
 * isCurrencyDisplayEnabled() → false
 *
 * @remarks
 * Use this to conditionally show currency-specific UI elements
 */
export function isCurrencyDisplayEnabled(): boolean {
  const { meta } = getCurrencyDisplay()
  return meta.kind !== 'tokens'
}

/**
 * Format an amount that is ALREADY in local currency.
 *
 * This function does not apply any extra currency conversion. Use it for values
 * that are already in the payment currency.
 *
 * @param amount - Amount already in local currency units
 * @param options - Optional formatting configuration
 * @returns Formatted string with appropriate currency symbol
 *
 * @example
 * // Payment amount already calculated: ¥10 × priceRatio(1) = ¥10
 * // With quotaDisplayType: 'CNY'
 * formatLocalCurrencyAmount(10) → "¥10"
 *
 * @remarks
 * Use this function for:
 * - Payment amounts calculated via priceRatio (amount × price)
 * - Actual money charged to user's payment method
 * - Values that are already in the target currency
 *
 * DO NOT use for:
 * - CNY quota amounts → use formatCurrencyFromCNY()
 * - Raw quota values → use formatQuotaWithCurrency()
 *
 * Common mistake:
 * ```ts
 * const payment = cnyAmount * priceRatio
 * formatLocalCurrencyAmount(payment)
 * ```
 */
export function formatLocalCurrencyAmount(
  amount: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (amount == null || Number.isNaN(amount)) return '-'

  const { config } = getCurrencyDisplay()
  const meta = getBillingDisplayMeta(config)
  const merged = mergeOptions(options)

  return formatCurrencyValue(amount, merged, meta)
}
