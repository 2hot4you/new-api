/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ExternalLink, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { JsonEditor } from '@/components/json-editor'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'

import { createModel, getModel, getVendors, updateModel } from '../../api'
import { ENDPOINT_TEMPLATES, getNameRuleOptions } from '../../constants'
import {
  type ModelFormValues,
  modelFormSchema,
  modelsQueryKeys,
  transformFormDataToModelPayload,
  transformModelToFormDefaults,
  vendorsQueryKeys,
} from '../../lib'
import type { Model } from '../../types'
import { ModelCapabilityFields } from './model-capability-fields'
import { ModelMarketplaceFields } from './model-marketplace-fields'
import { ModelPublicationStatus } from './model-publication-status'

const EMPTY_FORM_VALUES: ModelFormValues = {
  model_name: '',
  display_name: '',
  description: '',
  description_en: '',
  icon: '',
  tags: [],
  vendor_id: undefined,
  endpoints: '',
  name_rule: 0,
  status: true,
  sync_official: true,
  marketplace_enabled: false,
  context_length: 0,
  max_output_tokens: 0,
  knowledge_cutoff: '',
  release_date: '',
  input_modalities: [],
  output_modalities: [],
  capabilities: [],
  supported_parameters: [],
  supported_resolutions: [],
  supported_aspect_ratios: [],
  max_input_images: 0,
  output_formats: [],
  min_duration: 0,
  max_duration: 0,
  reference_modalities: [],
  enable_groups: [],
  quota_types: [],
}

type ModelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Model | null
}

function publicationState(values: ModelFormValues) {
  const result = modelFormSchema.safeParse({
    ...values,
    marketplace_enabled: true,
  })
  if (result.success) return { complete: true, missingFields: [] as string[] }

  return {
    complete: false,
    missingFields: [
      ...new Set(
        result.error.issues
          .map((issue) => String(issue.path[0] ?? ''))
          .filter(Boolean)
      ),
    ],
  }
}

export function ModelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ModelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentModelId = currentRow?.id
  const isEditing = Boolean(currentModelId)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)

  const form = useForm<ModelFormValues>({
    resolver: zodResolver(modelFormSchema),
    defaultValues: EMPTY_FORM_VALUES,
  })

  const { data: vendorsData } = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: open,
  })
  const vendors = vendorsData?.data?.items ?? []

  const { data: modelResponse, isLoading: isModelLoading } = useQuery({
    queryKey: modelsQueryKeys.detail(currentModelId ?? 0),
    queryFn: () => {
      if (!currentModelId) throw new Error('Model ID is required')
      return getModel(currentModelId)
    },
    enabled: open && isEditing,
  })
  const model = modelResponse?.data ?? currentRow ?? undefined

  useEffect(() => {
    if (!open) return
    if (isEditing) {
      if (modelResponse?.data) {
        form.reset(transformModelToFormDefaults(modelResponse.data))
      }
      return
    }

    form.reset({
      ...EMPTY_FORM_VALUES,
      model_name: currentRow?.model_name ?? '',
      tags: [],
      input_modalities: [],
      output_modalities: [],
      capabilities: [],
      supported_parameters: [],
      supported_resolutions: [],
      supported_aspect_ratios: [],
      output_formats: [],
      reference_modalities: [],
      enable_groups: [],
      quota_types: [],
    })
  }, [currentRow, form, isEditing, modelResponse, open])

  const values = form.watch()
  const localPublication = useMemo(() => publicationState(values), [values])
  const blockers = model?.marketplace_blockers ?? []
  const visible = Boolean(
    values.marketplace_enabled &&
    values.status &&
    localPublication.complete &&
    blockers.length === 0
  )
  const pricingConfigured = Boolean(
    model && !blockers.includes('pricing_missing')
  )

  async function onSubmit(formValues: ModelFormValues) {
    setIsSubmitting(true)
    try {
      const payload = transformFormDataToModelPayload(formValues)
      const response =
        isEditing && currentModelId
          ? await updateModel({ ...payload, id: currentModelId })
          : await createModel(payload)

      if (!response.success) {
        toast.error(response.message || t('Operation failed'))
        return
      }

      toast.success(
        isEditing
          ? t('Model updated successfully')
          : t('Model created successfully')
      )
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: ['pricing'] }),
        currentModelId
          ? queryClient.invalidateQueries({
              queryKey: modelsQueryKeys.detail(currentModelId),
            })
          : Promise.resolve(),
      ])
      onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing ? t('Edit Model') : t('Create Model')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Configure the model once here, then explicitly publish it to the public model marketplace.'
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='model-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            {isModelLoading ? (
              <div className='flex min-h-48 items-center justify-center'>
                <Loader2 className='text-muted-foreground size-5 animate-spin' />
              </div>
            ) : (
              <>
                <ModelMarketplaceFields
                  control={form.control}
                  vendors={vendors}
                  isEditing={isEditing}
                />

                <ModelCapabilityFields control={form.control} />

                <ModelPublicationStatus
                  enabled={values.marketplace_enabled}
                  onEnabledChange={(enabled) =>
                    form.setValue('marketplace_enabled', enabled, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                  }
                  modelEnabled={values.status}
                  onModelEnabledChange={(enabled) =>
                    form.setValue('status', enabled, { shouldDirty: true })
                  }
                  complete={localPublication.complete}
                  missingFields={localPublication.missingFields}
                  visible={visible}
                  blockers={blockers}
                  withdrawn={Boolean(model?.marketplace_withdrawn)}
                />

                <SideDrawerSection>
                  <div className='space-y-1'>
                    <h3 className='text-sm font-semibold'>{t('Pricing')}</h3>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Pricing is managed centrally and is read-only in model metadata.'
                      )}
                    </p>
                  </div>
                  <div className='flex items-center justify-between gap-4 rounded-lg border p-3'>
                    <div>
                      <div className='text-sm font-medium'>
                        {pricingConfigured
                          ? t('Pricing configured')
                          : t('Pricing not configured')}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {t('Quota types')}:{' '}
                        {values.quota_types.join(', ') || '—'}
                      </div>
                    </div>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        window.open(
                          '/system-settings/billing/model-pricing',
                          '_blank',
                          'noopener,noreferrer'
                        )
                      }
                    >
                      {t('Open model pricing')}
                      <ExternalLink className='ml-2 size-4' />
                    </Button>
                  </div>
                </SideDrawerSection>

                <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                  <SideDrawerSection>
                    <CollapsibleTrigger
                      render={
                        <button
                          type='button'
                          className='hover:bg-muted/40 flex w-full items-center justify-between rounded-md px-0 py-2 text-left transition-colors'
                        />
                      }
                    >
                      {t('Advanced routing')}
                      <ChevronDown
                        className={`size-4 transition-transform ${advancedOpen ? 'rotate-180' : ''}`}
                      />
                    </CollapsibleTrigger>
                    <CollapsibleContent className='space-y-4 pt-4'>
                      <FormField
                        control={form.control}
                        name='name_rule'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Name Rule')}</FormLabel>
                            <Select
                              items={getNameRuleOptions(t).map((option) => ({
                                label: option.label,
                                value: String(option.value),
                              }))}
                              value={String(field.value)}
                              onValueChange={(value) =>
                                field.onChange(
                                  Number.parseInt(value ?? '0', 10)
                                )
                              }
                            >
                              <FormControl>
                                <SelectTrigger>
                                  <SelectValue />
                                </SelectTrigger>
                              </FormControl>
                              <SelectContent alignItemWithTrigger={false}>
                                <SelectGroup>
                                  {getNameRuleOptions(t).map((option) => (
                                    <SelectItem
                                      key={option.value}
                                      value={String(option.value)}
                                    >
                                      {option.label}
                                    </SelectItem>
                                  ))}
                                </SelectGroup>
                              </SelectContent>
                            </Select>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='endpoints'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Endpoints')}</FormLabel>
                            <FormControl>
                              <JsonEditor
                                value={field.value}
                                onChange={field.onChange}
                                keyPlaceholder={t('Endpoint type')}
                                valuePlaceholder={t('Endpoint configuration')}
                                valueType='any'
                              />
                            </FormControl>
                            <div className='flex flex-wrap gap-2'>
                              {Object.keys(ENDPOINT_TEMPLATES).map(
                                (endpoint) => (
                                  <Button
                                    key={endpoint}
                                    type='button'
                                    size='sm'
                                    variant='outline'
                                    onClick={() =>
                                      field.onChange(
                                        JSON.stringify(
                                          {
                                            [endpoint]:
                                              ENDPOINT_TEMPLATES[endpoint],
                                          },
                                          null,
                                          2
                                        )
                                      )
                                    }
                                  >
                                    {endpoint}
                                  </Button>
                                )
                              )}
                            </div>
                            <FormDescription>
                              {t(
                                'Custom API endpoint definitions for this model.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='sync_official'
                        render={({ field }) => (
                          <FormItem className={sideDrawerSwitchItemClassName()}>
                            <div>
                              <FormLabel>{t('Official Sync')}</FormLabel>
                              <FormDescription>
                                {t('Sync this model with official upstream')}
                              </FormDescription>
                            </div>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    </CollapsibleContent>
                  </SideDrawerSection>
                </Collapsible>
              </>
            )}
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' disabled={isSubmitting} />}
          >
            {t('Cancel')}
          </SheetClose>
          <Button
            form='model-form'
            type='submit'
            disabled={isSubmitting || isModelLoading}
          >
            {isSubmitting && <Loader2 className='mr-2 size-4 animate-spin' />}
            {isEditing ? t('Update Model') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
