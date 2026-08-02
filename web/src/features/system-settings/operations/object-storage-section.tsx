/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const createObjectStorageSchema = (t: (key: string) => string) =>
  z.object({
    COSEnabled: z.boolean(),
    COSBucket: z.string().trim(),
    COSRegion: z.string().trim(),
    COSSecretID: z.string().trim(),
    COSSecretKey: z.string(),
    COSCustomDomain: z
      .string()
      .trim()
      .refine((value) => !value || /^https:\/\/[^/]+\/?$/.test(value), {
        message: t('Use an HTTPS origin without a path'),
      }),
    COSPathPrefix: z
      .string()
      .trim()
      .min(1)
      .regex(/^[a-zA-Z0-9/_-]+$/, {
        message: t('Use letters, numbers, slashes, underscores, or hyphens'),
      }),
    COSUploadExpiryMinutes: z.number().int().min(5).max(120),
    COSReadExpiryMinutes: z.number().int().min(5).max(1440),
  })

type ObjectStorageFormValues = z.infer<
  ReturnType<typeof createObjectStorageSchema>
>

type ObjectStorageSectionProps = {
  defaultValues: ObjectStorageFormValues
  secretConfigured: boolean
}

export function ObjectStorageSection(props: ObjectStorageSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = createObjectStorageSchema(t)
  const form = useForm<ObjectStorageFormValues>({
    resolver: zodResolver(schema),
    defaultValues: props.defaultValues,
  })

  useResetForm(form, props.defaultValues)

  const onSubmit = async (values: ObjectStorageFormValues) => {
    const updates: Array<{ key: string; value: string | number | boolean }> = []
    const comparableKeys = [
      'COSBucket',
      'COSRegion',
      'COSSecretID',
      'COSCustomDomain',
      'COSPathPrefix',
      'COSUploadExpiryMinutes',
      'COSReadExpiryMinutes',
    ] as const

    for (const key of comparableKeys) {
      if (values[key] !== props.defaultValues[key]) {
        updates.push({ key, value: values[key] })
      }
    }
    if (values.COSSecretKey.trim()) {
      updates.push({ key: 'COSSecretKey', value: values.COSSecretKey.trim() })
    }
    if (values.COSEnabled !== props.defaultValues.COSEnabled) {
      updates.push({ key: 'COSEnabled', value: values.COSEnabled })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    form.setValue('COSSecretKey', '')
  }

  const testConnection = async () => {
    if (!(await form.trigger())) return
    try {
      await onSubmit(form.getValues())
      const response = await api.post('/api/assets/admin/cos/test')
      if (response.data?.success) {
        toast.success(t('COS connection successful'))
      } else {
        toast.error(response.data?.message || t('COS connection failed'))
      }
    } catch (error) {
      const message =
        error && typeof error === 'object' && 'response' in error
          ? (error.response as { data?: { message?: string } })?.data?.message
          : undefined
      toast.error(message || t('COS connection failed'))
    }
  }

  return (
    <SettingsSection title={t('Object Storage')}>
      <Alert>
        <AlertDescription>
          {t(
            'Files are uploaded directly from the browser to a private Tencent COS bucket. Configure bucket CORS for PUT, GET, and HEAD from the Molii web origin.'
          )}
        </AlertDescription>
      </Alert>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save object storage settings'
          />
          <FormField
            control={form.control}
            name='COSEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Tencent COS uploads')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When disabled, users can still create temporary assets from public URLs.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
          {[
            ['COSBucket', 'Bucket', 'molii-assets-1250000000'],
            ['COSRegion', 'Region', 'ap-guangzhou'],
            ['COSSecretID', 'SecretId', 'AKID...'],
          ].map(([name, label, placeholder]) => (
            <FormField
              key={name}
              control={form.control}
              name={name as 'COSBucket' | 'COSRegion' | 'COSSecretID'}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t(label)}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={placeholder}
                      autoComplete='off'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          ))}
          <FormField
            control={form.control}
            name='COSSecretKey'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('SecretKey')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    autoComplete='new-password'
                    placeholder={
                      props.secretConfigured
                        ? t('Configured; enter a new value to replace it')
                        : t('Enter COS SecretKey')
                    }
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The SecretKey is never returned to the browser after saving.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='COSCustomDomain'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Custom origin domain')}</FormLabel>
                <FormControl>
                  <Input
                    type='url'
                    placeholder='https://assets.molii.co'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional. Use a COS custom origin domain, not a CDN domain, because COS signatures include the request host.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='COSPathPrefix'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Object key prefix')}</FormLabel>
                <FormControl>
                  <Input placeholder='users' {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Objects use the pattern prefix/userId/starai-assets/type/year/month/uuid.ext.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          {[
            ['COSUploadExpiryMinutes', 'Upload URL validity (minutes)', 30],
            ['COSReadExpiryMinutes', 'Read URL validity (minutes)', 60],
          ].map(([name, label, placeholder]) => (
            <FormField
              key={name}
              control={form.control}
              name={name as 'COSUploadExpiryMinutes' | 'COSReadExpiryMinutes'}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t(String(label))}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      inputMode='numeric'
                      placeholder={String(placeholder)}
                      {...field}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          ))}
          <div data-settings-form-span='full'>
            <Button
              type='button'
              variant='outline'
              disabled={updateOption.isPending}
              onClick={() => void testConnection()}
            >
              {t('Test COS connection')}
            </Button>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
