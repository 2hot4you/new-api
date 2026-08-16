/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { type Control, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { SideDrawerSection } from '@/components/drawer-layout'
import { TagInput } from '@/components/tag-input'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import {
  inferMarketplaceCategory,
  MODEL_CAPABILITY_OPTIONS,
  MODEL_MODALITY_OPTIONS,
  MODEL_OUTPUT_FORMAT_OPTIONS,
  MODEL_PARAMETER_OPTIONS,
  MODEL_REFERENCE_MODALITY_OPTIONS,
  type ModelFormValues,
} from '../../lib/model-form'

function CatalogOptionPicker({
  value,
  options,
  onChange,
}: {
  value: string[]
  options: readonly string[]
  onChange: (value: string[]) => void
}) {
  return (
    <div className='flex flex-wrap gap-2'>
      {options.map((option) => {
        const selected = value.includes(option)
        return (
          <Button
            key={option}
            type='button'
            size='sm'
            variant={selected ? 'default' : 'outline'}
            aria-pressed={selected}
            onClick={() =>
              onChange(
                selected
                  ? value.filter((item) => item !== option)
                  : [...value, option]
              )
            }
          >
            {option}
          </Button>
        )
      })}
    </div>
  )
}

function NumberField({
  control,
  name,
  label,
  description,
}: {
  control: Control<ModelFormValues>
  name:
    | 'context_length'
    | 'max_output_tokens'
    | 'max_input_images'
    | 'min_duration'
    | 'max_duration'
  label: string
  description?: string
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input
              type='number'
              min={0}
              step={1}
              value={field.value}
              onChange={(event) =>
                field.onChange(
                  event.target.value === ''
                    ? 0
                    : Number.parseInt(event.target.value, 10)
                )
              }
            />
          </FormControl>
          {description && <FormDescription>{description}</FormDescription>}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

export function ModelCapabilityFields({
  control,
}: {
  control: Control<ModelFormValues>
}) {
  const { t } = useTranslation()
  const [capabilities = [], outputModalities = []] = useWatch({
    control,
    name: ['capabilities', 'output_modalities'],
  })
  const category = inferMarketplaceCategory({
    capabilities,
    output_modalities: outputModalities,
  })

  return (
    <SideDrawerSection>
      <div className='space-y-1'>
        <h3 className='text-sm font-semibold'>{t('Capabilities')}</h3>
        <p className='text-muted-foreground text-xs'>
          {t('Category')}: {category.toUpperCase()}
        </p>
      </div>

      <div className='grid gap-4 sm:grid-cols-2'>
        {(['input_modalities', 'output_modalities'] as const).map((name) => (
          <FormField
            key={name}
            control={control}
            name={name}
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t(
                    name === 'input_modalities'
                      ? 'Input modalities'
                      : 'Output modalities'
                  )}
                </FormLabel>
                <FormControl>
                  <CatalogOptionPicker
                    value={field.value}
                    options={MODEL_MODALITY_OPTIONS}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        ))}
      </div>

      <FormField
        control={control}
        name='capabilities'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Capabilities')}</FormLabel>
            <FormControl>
              <CatalogOptionPicker
                value={field.value}
                options={MODEL_CAPABILITY_OPTIONS}
                onChange={field.onChange}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      {category === 'llm' && (
        <>
          <FormField
            control={control}
            name='supported_parameters'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Supported parameters')}</FormLabel>
                <FormControl>
                  <CatalogOptionPicker
                    value={field.value}
                    options={MODEL_PARAMETER_OPTIONS}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <div className='grid gap-4 sm:grid-cols-2'>
            <NumberField
              control={control}
              name='context_length'
              label={t('Context length')}
              description={t('Maximum input context window in tokens')}
            />
            <NumberField
              control={control}
              name='max_output_tokens'
              label={t('Maximum output tokens')}
              description={t('Maximum tokens generated in one response')}
            />
          </div>
        </>
      )}

      {(category === 'image' || category === 'video') && (
        <div className='grid gap-4 sm:grid-cols-2'>
          <FormField
            control={control}
            name='supported_resolutions'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Supported resolutions')}</FormLabel>
                <FormControl>
                  <TagInput
                    value={field.value}
                    onChange={field.onChange}
                    placeholder='480p, 720p, 1080p, 1k, 2k'
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={control}
            name='supported_aspect_ratios'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Supported aspect ratios')}</FormLabel>
                <FormControl>
                  <TagInput
                    value={field.value}
                    onChange={field.onChange}
                    placeholder='16:9, 9:16, 1:1'
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      )}

      {category === 'image' && (
        <>
          <FormField
            control={control}
            name='output_formats'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Output formats')}</FormLabel>
                <FormControl>
                  <CatalogOptionPicker
                    value={field.value}
                    options={MODEL_OUTPUT_FORMAT_OPTIONS}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          {capabilities.includes('image_editing') && (
            <NumberField
              control={control}
              name='max_input_images'
              label={t('Maximum input images')}
            />
          )}
        </>
      )}

      {category === 'video' && (
        <>
          <div className='grid gap-4 sm:grid-cols-2'>
            <NumberField
              control={control}
              name='min_duration'
              label={t('Minimum duration (seconds)')}
            />
            <NumberField
              control={control}
              name='max_duration'
              label={t('Maximum duration (seconds)')}
            />
          </div>
          <FormField
            control={control}
            name='reference_modalities'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Reference modalities')}</FormLabel>
                <FormControl>
                  <CatalogOptionPicker
                    value={field.value}
                    options={MODEL_REFERENCE_MODALITY_OPTIONS}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </>
      )}
    </SideDrawerSection>
  )
}
