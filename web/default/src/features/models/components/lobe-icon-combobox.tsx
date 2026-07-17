import iconToc from '@lobehub/icons/es/toc.js'
import { useMemo } from 'react'
import {
  ComboboxInput,
  type ComboboxInputOption,
} from '@/components/ui/combobox-input'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

type LobeIconTocItem = {
  fullTitle?: string
  id: string
  param?: {
    hasAvatar?: boolean
    hasColor?: boolean
    hasCombine?: boolean
    hasText?: boolean
    hasTextCn?: boolean
  }
  title?: string
}

type LobeIconComboboxProps = {
  className?: string
  emptyText?: string
  id?: string
  onValueChange: (value: string) => void
  placeholder?: string
  value?: string
}

const COMMON_ICON_IDS = [
  'OpenAI',
  'Claude',
  'Gemini',
  'Qwen',
  'DeepSeek',
  'Doubao',
  'Minimax',
  'Vidu',
  'Kling',
  'Jimeng',
  'Moonshot',
  'Zhipu',
  'Wenxin',
  'Hunyuan',
  'SiliconCloud',
]

function getIconVariants(icon: LobeIconTocItem) {
  const variants = [
    {
      label: 'Mono',
      value: icon.id,
    },
  ]

  if (icon.param?.hasColor) {
    variants.push({
      label: 'Color',
      value: `${icon.id}.Color`,
    })
  }

  if (icon.param?.hasText) {
    variants.push({
      label: 'Text',
      value: `${icon.id}.Text`,
    })
  }

  if (icon.param?.hasTextCn) {
    variants.push({
      label: 'TextCn',
      value: `${icon.id}.TextCn`,
    })
  }

  if (icon.param?.hasCombine) {
    variants.push({
      label: 'Combine',
      value: `${icon.id}.Combine`,
    })
  }

  if (icon.param?.hasAvatar) {
    variants.push({
      label: 'Avatar',
      value: `${icon.id}.Avatar`,
    })
  }

  return variants
}

const lobeIconOptions: ComboboxInputOption[] = (
  iconToc as LobeIconTocItem[]
)
  .flatMap((icon) => {
    const title = icon.fullTitle || icon.title || icon.id

    return getIconVariants(icon).map((variant) => ({
      value: variant.value,
      label: `${title} - ${variant.label} (${variant.value})`,
      icon: getLobeIcon(variant.value, 16),
    }))
  })
  .sort((a, b) => {
    const aBase = a.value.split('.')[0]
    const bBase = b.value.split('.')[0]
    const aCommonIndex = COMMON_ICON_IDS.indexOf(aBase)
    const bCommonIndex = COMMON_ICON_IDS.indexOf(bBase)

    if (aCommonIndex !== -1 || bCommonIndex !== -1) {
      if (aCommonIndex === -1) return 1
      if (bCommonIndex === -1) return -1
      return aCommonIndex - bCommonIndex
    }

    return a.label.localeCompare(b.label)
  })

const commonLobeIconOptions = lobeIconOptions.filter((option) =>
  COMMON_ICON_IDS.includes(option.value.split('.')[0])
)

export function LobeIconCombobox({
  className,
  emptyText,
  id,
  onValueChange,
  placeholder,
  value = '',
}: LobeIconComboboxProps) {
  const options = useMemo(
    () => (value.trim() ? lobeIconOptions : commonLobeIconOptions),
    [value]
  )

  return (
    <div className={cn('flex items-center gap-2', className)}>
      <div className='border-input bg-muted/30 flex size-8 shrink-0 items-center justify-center rounded-lg border'>
        {getLobeIcon(value, 18)}
      </div>
      <ComboboxInput
        id={id}
        options={options}
        value={value}
        onValueChange={onValueChange}
        placeholder={placeholder}
        emptyText={emptyText}
      />
    </div>
  )
}
