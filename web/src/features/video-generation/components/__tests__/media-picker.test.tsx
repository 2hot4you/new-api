/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import type { TemporaryAsset } from '@/features/temporary-assets/lib/asset-utils'

import { VideoStudioMediaPicker } from '../media-picker'

const assets: TemporaryAsset[] = [
  {
    id: 'asset-image-old',
    asset_type: 'image',
    name: '旧场景',
    preview_url: 'https://cdn.example/old.png',
    status: 'ACTIVE',
    created_at: 10,
    expires_at: 9999999999,
    verified_at: 11,
  },
  {
    id: 'asset-video-new',
    asset_type: 'video',
    name: '最新运镜',
    preview_url: 'https://cdn.example/new.mp4',
    status: 'ACTIVE',
    created_at: 30,
    expires_at: 9999999999,
    verified_at: 31,
  },
  {
    id: 'asset-image-failed',
    asset_type: 'image',
    name: '审核失败图片',
    status: 'FAILED',
    created_at: 20,
    expires_at: 9999999999,
    verified_at: 21,
    error: { message: '审核未通过' },
  },
]

test('uses two explicit image slots for first and last frame mode', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()

  render(
    <VideoStudioMediaPicker
      mode='frames'
      media={[]}
      assets={assets}
      onChange={onChange}
      onAssetsChanged={vi.fn()}
    />
  )

  expect(
    screen.getByRole('button', { name: 'Choose first frame' })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Choose last frame' })
  ).toBeInTheDocument()
  expect(screen.queryByRole('combobox', { name: 'Frame role' })).toBeNull()

  await user.click(screen.getByRole('button', { name: 'Choose first frame' }))

  const dialog = screen.getByRole('dialog', { name: 'Choose first frame' })
  expect(within(dialog).getByText('旧场景')).toBeInTheDocument()
  expect(within(dialog).queryByText('最新运镜')).not.toBeInTheDocument()
  expect(
    within(dialog).getByRole('option', { name: /审核失败图片/ })
  ).toBeDisabled()

  await user.click(within(dialog).getByRole('option', { name: /旧场景/ }))

  expect(onChange).toHaveBeenCalledWith([
    expect.objectContaining({
      type: 'image',
      source: 'asset',
      role: 'first_frame',
      value: 'asset-image-old',
    }),
  ])
})

test('adds a public image URL directly to the active frame slot', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()

  render(
    <VideoStudioMediaPicker
      mode='frames'
      media={[]}
      assets={[]}
      onChange={onChange}
      onAssetsChanged={vi.fn()}
    />
  )

  await user.click(screen.getByRole('button', { name: 'Choose last frame' }))
  const dialog = screen.getByRole('dialog', { name: 'Choose last frame' })
  await user.type(
    within(dialog).getByRole('textbox', { name: 'Public media URL' }),
    'https://cdn.example.com/last-frame.png'
  )
  await user.click(within(dialog).getByRole('button', { name: 'Add URL' }))

  expect(onChange).toHaveBeenCalledWith([
    expect.objectContaining({
      type: 'image',
      source: 'url',
      role: 'last_frame',
      value: 'https://cdn.example.com/last-frame.png',
    }),
  ])
})

test('selects multiple categorized reference assets in newest-first order', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()

  render(
    <VideoStudioMediaPicker
      mode='references'
      media={[]}
      assets={assets}
      onChange={onChange}
      onAssetsChanged={vi.fn()}
    />
  )

  await user.click(screen.getByRole('button', { name: 'Add reference media' }))
  const dialog = screen.getByRole('dialog', { name: 'Choose reference media' })
  const options = within(dialog).getAllByRole('option')
  expect(options[0]).toHaveAccessibleName(/最新运镜/)

  await user.click(within(dialog).getByRole('option', { name: /最新运镜/ }))
  await user.click(within(dialog).getByRole('option', { name: /旧场景/ }))
  await user.click(
    within(dialog).getByRole('button', { name: 'Add selected (2)' })
  )

  expect(onChange).toHaveBeenCalledWith([
    expect.objectContaining({
      type: 'video',
      role: 'reference_video',
      value: 'asset-video-new',
      mentionIndex: 1,
    }),
    expect.objectContaining({
      type: 'image',
      role: 'reference_image',
      value: 'asset-image-old',
      mentionIndex: 1,
    }),
  ])
})

test('does not expose media inputs in text mode', () => {
  render(
    <VideoStudioMediaPicker
      mode='text'
      media={[]}
      assets={[]}
      onChange={vi.fn()}
      onAssetsChanged={vi.fn()}
    />
  )

  expect(
    screen.getByText('Text mode does not include reference media.')
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Add reference media' })
  ).not.toBeInTheDocument()
})
