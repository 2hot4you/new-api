/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import {
  useWatch,
  type ControllerRenderProps,
  type Resolver,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { CLAUDEYE_WORDMARK_FALLBACK } from '@/components/layout/components/claudeye-wordmark'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  BRAND_HEX_PATTERN,
  buildClaudeyePreviewUrl,
  CLAUDEYE_BRAND_DEFAULTS,
  contrastRatio,
  normalizeBrandHex,
  type ClaudeyeBrandColorKey,
  type ClaudeyeBrandSurface,
  type ClaudeyePreviewColors,
} from './claudeye-brand-colors'

export type ClaudeyeBrandValues = Record<ClaudeyeBrandColorKey, string>

type ClaudeyeBrandFormValues = {
  brand_setting: {
    claudeye_light_mark_color: string
    claudeye_light_text_color: string
    claudeye_dark_mark_color: string
    claudeye_dark_text_color: string
  }
}

type ClaudeyeBrandFieldName =
  | 'brand_setting.claudeye_light_mark_color'
  | 'brand_setting.claudeye_light_text_color'
  | 'brand_setting.claudeye_dark_mark_color'
  | 'brand_setting.claudeye_dark_text_color'

type BrandFieldDescriptor = {
  name: ClaudeyeBrandFieldName
  label: string
}

const LIGHT_FIELDS: readonly BrandFieldDescriptor[] = [
  {
    name: 'brand_setting.claudeye_light_mark_color',
    label: 'Light surface mark color',
  },
  {
    name: 'brand_setting.claudeye_light_text_color',
    label: 'Light surface wordmark color',
  },
]

const DARK_FIELDS: readonly BrandFieldDescriptor[] = [
  {
    name: 'brand_setting.claudeye_dark_mark_color',
    label: 'Dark surface mark color',
  },
  {
    name: 'brand_setting.claudeye_dark_text_color',
    label: 'Dark surface wordmark color',
  },
]

const ALL_FIELDS = [...LIGHT_FIELDS, ...DARK_FIELDS] as const

function nestedDefaults(values: ClaudeyeBrandValues): ClaudeyeBrandFormValues {
  const read = (key: ClaudeyeBrandColorKey) =>
    normalizeBrandHex(values[key]) ?? CLAUDEYE_BRAND_DEFAULTS[key]

  return {
    brand_setting: {
      claudeye_light_mark_color: read(
        'brand_setting.claudeye_light_mark_color'
      ),
      claudeye_light_text_color: read(
        'brand_setting.claudeye_light_text_color'
      ),
      claudeye_dark_mark_color: read('brand_setting.claudeye_dark_mark_color'),
      claudeye_dark_text_color: read('brand_setting.claudeye_dark_text_color'),
    },
  }
}

function ColorField(props: {
  field: ControllerRenderProps<ClaudeyeBrandFormValues, ClaudeyeBrandFieldName>
  label: string
  onValueChange: (name: ClaudeyeBrandFieldName, value: string) => void
}) {
  const { t } = useTranslation()
  const initialColor = normalizeBrandHex(props.field.value) ?? '#000000'
  const [lastValidColor, setLastValidColor] = useState(initialColor)
  const normalized = normalizeBrandHex(props.field.value)
  const pickerValue = normalized ?? lastValidColor

  const updateHex = (value: string) => {
    props.field.onChange(value)
    props.onValueChange(props.field.name, value)
    const nextColor = normalizeBrandHex(value)
    if (nextColor) setLastValidColor(nextColor)
  }

  return (
    <FormItem>
      <FormLabel>{t(props.label)}</FormLabel>
      <div className='flex items-center gap-3'>
        <Input
          type='color'
          aria-label={`${t(props.label)} picker`}
          value={pickerValue}
          onChange={(event) => updateHex(event.target.value.toUpperCase())}
          className='size-8 shrink-0 cursor-pointer p-1'
        />
        <FormControl>
          <Input
            type='text'
            value={props.field.value}
            onChange={(event) => updateHex(event.target.value)}
            onBlur={props.field.onBlur}
            name={props.field.name}
            className='font-mono uppercase'
          />
        </FormControl>
        <span
          aria-hidden
          data-brand-color-swatch
          className='size-8 shrink-0 rounded-md border'
          style={{ backgroundColor: pickerValue }}
        />
      </div>
      <FormMessage />
    </FormItem>
  )
}

function BrandPreview(props: {
  backgroundColor: string
  colors: ClaudeyePreviewColors
  label: string
  surface: ClaudeyeBrandSurface
}) {
  const { t } = useTranslation()
  const [useFallback, setUseFallback] = useState(false)
  const previewUrl = buildClaudeyePreviewUrl(
    props.surface,
    props.colors.mark,
    props.colors.text,
    props.colors
  )
  const lowContrast =
    contrastRatio(props.colors.mark, props.backgroundColor) < 4.5 ||
    contrastRatio(props.colors.text, props.backgroundColor) < 4.5

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-3'>
        <p className='text-sm font-medium'>{t(props.label)}</p>
        {lowContrast ? (
          <p className='text-xs font-medium text-amber-600 dark:text-amber-400'>
            {t('Low contrast')}
          </p>
        ) : null}
      </div>
      <div
        className='flex min-h-24 items-center justify-center rounded-xl border p-5'
        style={{ backgroundColor: props.backgroundColor }}
      >
        <img
          src={useFallback ? CLAUDEYE_WORDMARK_FALLBACK : (previewUrl ?? '')}
          alt={t(props.label)}
          width={1995}
          height={440}
          className='h-auto max-h-14 w-full max-w-md object-contain'
          onError={useFallback ? undefined : () => setUseFallback(true)}
        />
      </div>
    </div>
  )
}

type ClaudeyeBrandAppearanceSectionProps = {
  defaultValues: ClaudeyeBrandValues
}

export function ClaudeyeBrandAppearanceSection({
  defaultValues,
}: ClaudeyeBrandAppearanceSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaults = nestedDefaults(defaultValues)
  const colorSchema = z.string().regex(BRAND_HEX_PATTERN, {
    error: () => t('Color must use #RRGGBB format'),
  })
  const schema = z.object({
    brand_setting: z.object({
      claudeye_light_mark_color: colorSchema,
      claudeye_light_text_color: colorSchema,
      claudeye_dark_mark_color: colorSchema,
      claudeye_dark_text_color: colorSchema,
    }),
  })

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<ClaudeyeBrandFormValues>({
      resolver: zodResolver(schema) as Resolver<
        ClaudeyeBrandFormValues,
        unknown,
        ClaudeyeBrandFormValues
      >,
      defaultValues: defaults,
      mode: 'onChange',
      onSubmit: async (_values, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          const normalized = normalizeBrandHex(String(value))
          if (!normalized) continue
          await updateOption.mutateAsync({ key, value: normalized })
        }
      },
    })

  const [lastLightPreview, setLastLightPreview] =
    useState<ClaudeyePreviewColors>({
      mark: defaults.brand_setting.claudeye_light_mark_color,
      text: defaults.brand_setting.claudeye_light_text_color,
    })
  const [lastDarkPreview, setLastDarkPreview] = useState<ClaudeyePreviewColors>(
    {
      mark: defaults.brand_setting.claudeye_dark_mark_color,
      text: defaults.brand_setting.claudeye_dark_text_color,
    }
  )
  const watchedColors = useWatch({
    control: form.control,
    name: 'brand_setting',
  })
  const allColorsValid = Object.values(watchedColors).every(
    (value) => normalizeBrandHex(value) !== null
  )

  const updatePreview = (name: ClaudeyeBrandFieldName, value: string) => {
    const current = form.getValues().brand_setting
    const settingName = name.replace(
      'brand_setting.',
      ''
    ) as keyof ClaudeyeBrandFormValues['brand_setting']
    const next = { ...current, [settingName]: value }

    if (name.includes('_light_')) {
      const mark = normalizeBrandHex(next.claudeye_light_mark_color)
      const text = normalizeBrandHex(next.claudeye_light_text_color)
      if (mark && text) setLastLightPreview({ mark, text })
      return
    }

    const mark = normalizeBrandHex(next.claudeye_dark_mark_color)
    const text = normalizeBrandHex(next.claudeye_dark_text_color)
    if (mark && text) setLastDarkPreview({ mark, text })
  }

  const restoreDefaults = () => {
    for (const field of ALL_FIELDS) {
      form.setValue(field.name, CLAUDEYE_BRAND_DEFAULTS[field.name], {
        shouldDirty: true,
        shouldTouch: true,
        shouldValidate: true,
      })
    }
    setLastLightPreview({
      mark: CLAUDEYE_BRAND_DEFAULTS['brand_setting.claudeye_light_mark_color'],
      text: CLAUDEYE_BRAND_DEFAULTS['brand_setting.claudeye_light_text_color'],
    })
    setLastDarkPreview({
      mark: CLAUDEYE_BRAND_DEFAULTS['brand_setting.claudeye_dark_mark_color'],
      text: CLAUDEYE_BRAND_DEFAULTS['brand_setting.claudeye_dark_text_color'],
    })
  }

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Brand appearance')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={restoreDefaults}
              resetLabel='Restore default colors'
              isSaving={isSubmitting || updateOption.isPending}
              isSaveDisabled={!isDirty || !allColorsValid}
            />

            <div className='grid gap-5 md:grid-cols-2'>
              {[...LIGHT_FIELDS, ...DARK_FIELDS].map((descriptor) => (
                <FormField
                  key={descriptor.name}
                  control={form.control}
                  name={descriptor.name}
                  render={({ field }) => (
                    <ColorField
                      field={field}
                      label={descriptor.label}
                      onValueChange={updatePreview}
                    />
                  )}
                />
              ))}
            </div>

            <div className='grid gap-5 md:grid-cols-2'>
              <BrandPreview
                key={`light-${lastLightPreview.mark}-${lastLightPreview.text}`}
                surface='light'
                label='Light header preview'
                backgroundColor='#FFFFFF'
                colors={lastLightPreview}
              />
              <BrandPreview
                key={`dark-${lastDarkPreview.mark}-${lastDarkPreview.text}`}
                surface='dark'
                label='Dark footer preview'
                backgroundColor='#171717'
                colors={lastDarkPreview}
              />
            </div>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
