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
import { Input } from '@/components/ui/input'

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

const schema = z.object({
  standard_720p: z.coerce.number().min(0),
  standard_720p_video: z.coerce.number().min(0),
  standard_1080p: z.coerce.number().min(0),
  standard_1080p_video: z.coerce.number().min(0),
  standard_4k: z.coerce.number().min(0),
  standard_4k_video: z.coerce.number().min(0),
  fast_720p: z.coerce.number().min(0),
  fast_720p_video: z.coerce.number().min(0),
})

type Values = z.infer<typeof schema>
type FieldName = keyof Values

const fields: Array<{ name: FieldName; label: string; description: string }> = [
  { name: 'standard_720p', label: 'Seedance 2.0 · 720p · Text/Image', description: 'No reference video input' },
  { name: 'standard_720p_video', label: 'Seedance 2.0 · 720p · Video', description: 'Contains reference video input' },
  { name: 'standard_1080p', label: 'Seedance 2.0 · 1080p · Text/Image', description: 'No reference video input' },
  { name: 'standard_1080p_video', label: 'Seedance 2.0 · 1080p · Video', description: 'Contains reference video input' },
  { name: 'standard_4k', label: 'Seedance 2.0 · 4K · Text/Image', description: 'No reference video input' },
  { name: 'standard_4k_video', label: 'Seedance 2.0 · 4K · Video', description: 'Contains reference video input' },
  { name: 'fast_720p', label: 'Seedance 2.0 Fast · 720p · Text/Image', description: 'No reference video input' },
  { name: 'fast_720p_video', label: 'Seedance 2.0 Fast · 720p · Video', description: 'Contains reference video input' },
]

export function StarAIVideoPricingSection({ defaultValues }: { defaultValues: Values }) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const { form, handleSubmit, isDirty, isSubmitting } = useSettingsForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values, unknown, Values>,
    defaultValues,
    onSubmit: async (_data, changedFields) => {
      for (const [key, value] of Object.entries(changedFields)) {
        await updateOption.mutateAsync({
          key: `starai_video_price.${key}`,
          value: value as number,
        })
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
    <SettingsSection title={t('Molii AIGC Video Pricing')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <FormDescription>
            {t('Direct platform price per 1M tokens. CNY and USD are treated as 1:1; no exchange-rate conversion is applied.')}
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
                      <Input
                        type='number'
                        min={0}
                        step='0.01'
                        value={field.value ?? ''}
                        onChange={onNumberChange(field.onChange)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>{t(item.description)} · ¥ / 1M tokens</FormDescription>
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
