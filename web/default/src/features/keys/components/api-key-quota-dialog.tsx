import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { adjustApiKeyQuota } from '../api'
import { formatApiKeyQuotaPrecise, parseApiKeyQuotaAmount } from '../lib/quota'
import type { ApiKey, ApiKeyQuotaAdjustMode } from '../types'

interface ApiKeyQuotaDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKey: ApiKey | null
  onSuccess: () => void
  initialMode?: ApiKeyQuotaAdjustMode
}

export function ApiKeyQuotaDialog(props: ApiKeyQuotaDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<ApiKeyQuotaAdjustMode>('add')
  const [amount, setAmount] = useState('')
  const [loading, setLoading] = useState(false)

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const currentQuota = props.apiKey?.remain_quota ?? 0
  const amountValue = Number.parseFloat(amount) || 0
  const quotaValue = parseApiKeyQuotaAmount(Math.abs(amountValue))
  const defaultMode = props.initialMode ?? 'add'

  useEffect(() => {
    if (props.open) setMode(defaultMode)
  }, [defaultMode, props.open])

  const getPreviewText = () => {
    if (!props.apiKey) return ''
    const nextQuota =
      mode === 'add' ? currentQuota + quotaValue : currentQuota - quotaValue
    const operator = mode === 'add' ? '+' : '-'
    return `${t('Current quota')}: ${formatApiKeyQuotaPrecise(currentQuota)}  ${operator}${formatApiKeyQuotaPrecise(quotaValue)} = ${formatApiKeyQuotaPrecise(nextQuota)}`
  }

  const reset = () => {
    setAmount('')
    setMode(defaultMode)
  }

  const handleConfirm = async () => {
    if (!props.apiKey || props.apiKey.unlimited_quota) return
    if (!amount || quotaValue <= 0) {
      toast.error(t('Amount must be greater than 0'))
      return
    }
    if (mode === 'subtract' && quotaValue > currentQuota) {
      toast.error('减少额度不能超过当前额度')
      return
    }

    setLoading(true)
    try {
      const result = await adjustApiKeyQuota({
        id: props.apiKey.id,
        mode,
        value: quotaValue,
      })
      if (result.success) {
        toast.success(t('Quota adjusted successfully'))
        reset()
        props.onOpenChange(false)
        props.onSuccess()
      } else {
        toast.error(result.message || t('Failed to adjust quota'))
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t('Failed to adjust quota'))
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    reset()
    props.onOpenChange(false)
  }

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) reset()
    props.onOpenChange(isOpen)
  }

  const placeholder = tokensOnly
    ? t('Enter amount in tokens')
    : t('Enter amount in {{currency}}', { currency: currencyLabel })

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Adjust Quota')}</DialogTitle>
          <DialogDescription>
            {t('Select an operation mode and enter the amount')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4'>
          <div className='bg-muted/40 rounded-md border px-3 py-2 text-sm'>
            <div className='text-muted-foreground'>{t('API Key')}</div>
            <div className='font-medium'>{props.apiKey?.name || '-'}</div>
          </div>

          <div className='text-muted-foreground text-sm'>
            {getPreviewText()}
          </div>

          <div className='space-y-2'>
            <Label>{t('Mode')}</Label>
            <div className='flex gap-1'>
              {(['add', 'subtract'] as const).map((item) => (
                <Button
                  key={item}
                  type='button'
                  variant='outline'
                  size='sm'
                  className={cn(
                    mode === item &&
                      'bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                  )}
                  onClick={() => {
                    setMode(item)
                    setAmount('')
                  }}
                >
                  {item === 'add' ? t('Add') : t('Subtract')}
                </Button>
              ))}
            </div>
          </div>

          <div className='space-y-2'>
            <Label>
              {t('Amount')} ({currencyLabel})
            </Label>
            <Input
              type='number'
              step={tokensOnly ? 1 : 0.000001}
              min={0}
              placeholder={placeholder}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleConfirm()
              }}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={handleCancel}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
