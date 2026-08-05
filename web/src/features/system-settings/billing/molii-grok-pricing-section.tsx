import { zodResolver } from '@hookform/resolvers/zod'
import type { ChangeEvent } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { PricingUnitInput } from './pricing-unit-input'

const schema = z.object({
  image_standard_input: z.coerce.number().min(0),
  image_standard_1k: z.coerce.number().min(0),
  image_standard_2k: z.coerce.number().min(0),
  image_quality_input: z.coerce.number().min(0),
  image_quality_1k: z.coerce.number().min(0),
  image_quality_2k: z.coerce.number().min(0),
  video_15_image_input: z.coerce.number().min(0),
  video_15_480p: z.coerce.number().min(0),
  video_15_720p: z.coerce.number().min(0),
  video_15_1080p: z.coerce.number().min(0),
  video_image_input: z.coerce.number().min(0),
  video_video_input: z.coerce.number().min(0),
  video_480p: z.coerce.number().min(0),
  video_720p: z.coerce.number().min(0),
  tool_web_search: z.coerce.number().min(0),
  tool_x_search: z.coerce.number().min(0),
  tool_code_execution: z.coerce.number().min(0),
  tool_attachment_search: z.coerce.number().min(0),
  tool_collections_search: z.coerce.number().min(0),
  tool_image_generation: z.coerce.number().min(0),
})

export type MoliiGrokPricingValues = z.infer<typeof schema>
type FieldName = keyof MoliiGrokPricingValues

const fields: Array<{
  name: FieldName
  label: string
  unit: string
  namespace: 'molii_grok_price' | 'molii_grok_tool_price'
  option: string
}> = [
  {
    name: 'image_standard_input',
    label: 'Image · Standard · Media input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_standard_input',
  },
  {
    name: 'image_standard_1k',
    label: 'Image · Standard · 1K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_standard_1k',
  },
  {
    name: 'image_standard_2k',
    label: 'Image · Standard · 2K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_standard_2k',
  },
  {
    name: 'image_quality_input',
    label: 'Image · Quality · Media input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_quality_input',
  },
  {
    name: 'image_quality_1k',
    label: 'Image · Quality · 1K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_quality_1k',
  },
  {
    name: 'image_quality_2k',
    label: 'Image · Quality · 2K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_quality_2k',
  },
  {
    name: 'video_15_image_input',
    label: 'Video 1.5 · Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'video_15_image_input',
  },
  {
    name: 'video_15_480p',
    label: 'Video 1.5 · 480p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_15_480p',
  },
  {
    name: 'video_15_720p',
    label: 'Video 1.5 · 720p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_15_720p',
  },
  {
    name: 'video_15_1080p',
    label: 'Video 1.5 · 1080p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_15_1080p',
  },
  {
    name: 'video_image_input',
    label: 'Video · Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'video_image_input',
  },
  {
    name: 'video_video_input',
    label: 'Video · Video input',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_video_input',
  },
  {
    name: 'video_480p',
    label: 'Video · 480p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_480p',
  },
  {
    name: 'video_720p',
    label: 'Video · 720p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_720p',
  },
  {
    name: 'tool_web_search',
    label: 'Tool · Web search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'web_search',
  },
  {
    name: 'tool_x_search',
    label: 'Tool · X search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'x_search',
  },
  {
    name: 'tool_code_execution',
    label: 'Tool · Code execution',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'code_execution',
  },
  {
    name: 'tool_attachment_search',
    label: 'Tool · Attachment search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'attachment_search',
  },
  {
    name: 'tool_collections_search',
    label: 'Tool · Collections search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'collections_search',
  },
  {
    name: 'tool_image_generation',
    label: 'Tool · Image generation',
    unit: '¥ / completed image',
    namespace: 'molii_grok_tool_price',
    option: 'image_generation',
  },
]

export function MoliiGrokPricingSection({
  defaultValues,
}: {
  defaultValues: MoliiGrokPricingValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<MoliiGrokPricingValues>({
      resolver: zodResolver(schema) as Resolver<
        MoliiGrokPricingValues,
        unknown,
        MoliiGrokPricingValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [name, value] of Object.entries(changedFields)) {
          const field = fields.find((item) => item.name === name)
          if (field) {
            await updateOption.mutateAsync({
              key: `${field.namespace}.${field.option}`,
              value: value as number,
            })
          }
        }
      },
    })
  const onNumberChange =
    (onChange: (value: number | '') => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }

  return (
    <SettingsSection title={t('Molii Grok Imagine Pricing')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <FormDescription>
            {t(
              'Direct CNY prices. Official USD catalog numbers are treated as CNY at 1:1 without exchange-rate conversion.'
            )}
          </FormDescription>
          <SettingsFormGrid>
            {fields.map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t(item.label)}</FormLabel>
                    <FormControl>
                      <PricingUnitInput
                        type='number'
                        min={0}
                        step='0.001'
                        unit={t(item.unit)}
                        value={field.value ?? ''}
                        onChange={onNumberChange(field.onChange)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
