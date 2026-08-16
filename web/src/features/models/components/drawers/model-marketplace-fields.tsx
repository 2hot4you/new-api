/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { SideDrawerSection } from '@/components/drawer-layout'
import { TagInput } from '@/components/tag-input'
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import type { ModelFormValues } from '../../lib/model-form'
import type { Vendor } from '../../types'

type ModelMarketplaceFieldsProps = {
  control: Control<ModelFormValues>
  vendors: Vendor[]
  isEditing: boolean
}

export function ModelMarketplaceFields({
  control,
  vendors,
  isEditing,
}: ModelMarketplaceFieldsProps) {
  const { t } = useTranslation()

  return (
    <SideDrawerSection>
      <div className='space-y-1'>
        <h3 className='text-sm font-semibold'>{t('Marketplace metadata')}</h3>
        <p className='text-muted-foreground text-xs'>
          {t('These fields are the source of truth for the public model page.')}
        </p>
      </div>

      <FormField
        control={control}
        name='model_name'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Model ID *')}</FormLabel>
            <FormControl>
              <Input
                placeholder='gpt-4.1, doubao-seedance-2-0-260128'
                disabled={isEditing}
                {...field}
              />
            </FormControl>
            <FormDescription>
              {isEditing
                ? t('The model ID cannot be changed after creation.')
                : t('The exact ID clients use in API requests.')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className='grid gap-4 sm:grid-cols-2'>
        <FormField
          control={control}
          name='display_name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Display name')}</FormLabel>
              <FormControl>
                <Input placeholder={t('Public model name')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={control}
          name='vendor_id'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Vendor')}</FormLabel>
              <Select
                items={vendors.map((vendor) => ({
                  value: String(vendor.id),
                  label: vendor.name,
                }))}
                onValueChange={(value) =>
                  field.onChange(value ? Number.parseInt(value, 10) : undefined)
                }
                value={field.value ? String(field.value) : undefined}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder={t('Select vendor')} />
                  </SelectTrigger>
                </FormControl>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {vendors.map((vendor) => (
                      <SelectItem key={vendor.id} value={String(vendor.id)}>
                        {vendor.name}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={control}
        name='description'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Chinese description')}</FormLabel>
            <FormControl>
              <Textarea
                placeholder={t('Describe the model for Chinese users...')}
                rows={4}
                {...field}
              />
            </FormControl>
            <FormDescription>
              {t('Required for publication and used as the locale fallback.')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={control}
        name='description_en'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('English description')}</FormLabel>
            <FormControl>
              <Textarea
                placeholder='Optional English description...'
                rows={3}
                {...field}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className='grid gap-4 sm:grid-cols-3'>
        <FormField
          control={control}
          name='icon'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Logo override')}</FormLabel>
              <FormControl>
                <Input placeholder='OpenAI, DeepSeek, Qwen' {...field} />
              </FormControl>
              <FormDescription>{t('@lobehub/icons key')}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={control}
          name='release_date'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Release date')}</FormLabel>
              <FormControl>
                <Input type='date' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={control}
          name='knowledge_cutoff'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Knowledge cutoff')}</FormLabel>
              <FormControl>
                <Input placeholder='2025-04' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={control}
        name='tags'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Tags')}</FormLabel>
            <FormControl>
              <TagInput
                value={field.value}
                onChange={field.onChange}
                placeholder={t('Add tags...')}
              />
            </FormControl>
            <FormDescription>
              {t('Press Enter or comma to add tags')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </SideDrawerSection>
  )
}
