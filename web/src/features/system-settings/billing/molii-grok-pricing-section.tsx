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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

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
  image_20_input: z.coerce.number().min(0),
  image_20_low_1k: z.coerce.number().min(0),
  image_20_low_2k: z.coerce.number().min(0),
  image_20_medium_1k: z.coerce.number().min(0),
  image_20_medium_2k: z.coerce.number().min(0),
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
type PricingMode = 'images' | 'videos' | 'tools'

const fields: Array<{
  name: FieldName
  label: string
  unit: string
  namespace: 'molii_grok_price' | 'molii_grok_tool_price'
  option: string
}> = [
  {
    name: 'image_standard_input',
    label: 'Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_standard_input',
  },
  {
    name: 'image_standard_1k',
    label: '1K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_standard_1k',
  },
  {
    name: 'image_standard_2k',
    label: '2K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_standard_2k',
  },
  {
    name: 'image_quality_input',
    label: 'Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_quality_input',
  },
  {
    name: 'image_quality_1k',
    label: '1K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_quality_1k',
  },
  {
    name: 'image_quality_2k',
    label: '2K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_quality_2k',
  },
  {
    name: 'image_20_input',
    label: 'Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_20_input',
  },
  {
    name: 'image_20_low_1k',
    label: 'Low · 1K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_20_low_1k',
  },
  {
    name: 'image_20_low_2k',
    label: 'Low · 2K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_20_low_2k',
  },
  {
    name: 'image_20_medium_1k',
    label: 'Medium · 1K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_20_medium_1k',
  },
  {
    name: 'image_20_medium_2k',
    label: 'Medium · 2K output',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'image_20_medium_2k',
  },
  {
    name: 'video_15_image_input',
    label: 'Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'video_15_image_input',
  },
  {
    name: 'video_15_480p',
    label: '480p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_15_480p',
  },
  {
    name: 'video_15_720p',
    label: '720p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_15_720p',
  },
  {
    name: 'video_15_1080p',
    label: '1080p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_15_1080p',
  },
  {
    name: 'video_image_input',
    label: 'Image input',
    unit: '¥ / image',
    namespace: 'molii_grok_price',
    option: 'video_image_input',
  },
  {
    name: 'video_video_input',
    label: 'Video input',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_video_input',
  },
  {
    name: 'video_480p',
    label: '480p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_480p',
  },
  {
    name: 'video_720p',
    label: '720p output',
    unit: '¥ / second',
    namespace: 'molii_grok_price',
    option: 'video_720p',
  },
  {
    name: 'tool_web_search',
    label: 'Web search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'web_search',
  },
  {
    name: 'tool_x_search',
    label: 'X search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'x_search',
  },
  {
    name: 'tool_code_execution',
    label: 'Code execution',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'code_execution',
  },
  {
    name: 'tool_attachment_search',
    label: 'Attachment search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'attachment_search',
  },
  {
    name: 'tool_collections_search',
    label: 'Collections search',
    unit: '¥ / 1K calls',
    namespace: 'molii_grok_tool_price',
    option: 'collections_search',
  },
  {
    name: 'tool_image_generation',
    label: 'Image generation',
    unit: '¥ / completed image',
    namespace: 'molii_grok_tool_price',
    option: 'image_generation',
  },
]

const pricingModes: Array<{
  id: PricingMode
  label: string
  models: Array<{ name: string; fields: FieldName[] }>
}> = [
  {
    id: 'images',
    label: 'Image generation · per image',
    models: [
      {
        name: 'grok-imagine-image',
        fields: [
          'image_standard_input',
          'image_standard_1k',
          'image_standard_2k',
        ],
      },
      {
        name: 'grok-imagine-image-quality',
        fields: ['image_quality_input', 'image_quality_1k', 'image_quality_2k'],
      },
      {
        name: 'grok-imagine-image-2.0',
        fields: [
          'image_20_input',
          'image_20_low_1k',
          'image_20_low_2k',
          'image_20_medium_1k',
          'image_20_medium_2k',
        ],
      },
    ],
  },
  {
    id: 'videos',
    label: 'Video generation · per second',
    models: [
      {
        name: 'grok-imagine-video',
        fields: [
          'video_image_input',
          'video_video_input',
          'video_480p',
          'video_720p',
        ],
      },
      {
        name: 'grok-imagine-video-1.5',
        fields: [
          'video_15_image_input',
          'video_15_480p',
          'video_15_720p',
          'video_15_1080p',
        ],
      },
    ],
  },
  {
    id: 'tools',
    label: 'Tool invocation · per call',
    models: [
      {
        name: 'Grok tools',
        fields: [
          'tool_web_search',
          'tool_x_search',
          'tool_code_execution',
          'tool_attachment_search',
          'tool_collections_search',
          'tool_image_generation',
        ],
      },
    ],
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
          <Tabs defaultValue='images' className='gap-6'>
            <TabsList className='grid h-auto w-full max-w-2xl grid-cols-3'>
              {pricingModes.map((mode) => (
                <TabsTrigger
                  key={mode.id}
                  value={mode.id}
                  className='whitespace-normal'
                >
                  {t(mode.label)}
                </TabsTrigger>
              ))}
            </TabsList>

            {pricingModes.map((mode) => (
              <TabsContent key={mode.id} value={mode.id} className='space-y-6'>
                {mode.models.map((model) => (
                  <div
                    key={model.name}
                    className='bg-card space-y-4 rounded-xl border p-4'
                  >
                    <h3
                      data-grok-pricing-model
                      className='font-mono text-sm font-semibold'
                    >
                      {t(model.name)}
                    </h3>
                    <SettingsFormGrid>
                      {model.fields.map((fieldName) => {
                        const item = fields.find(
                          (candidate) => candidate.name === fieldName
                        )
                        if (!item) return null
                        return (
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
                        )
                      })}
                    </SettingsFormGrid>
                  </div>
                ))}
              </TabsContent>
            ))}
          </Tabs>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
