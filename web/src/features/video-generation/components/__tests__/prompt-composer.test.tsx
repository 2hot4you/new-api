/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import type { TemporaryAsset } from '@/features/temporary-assets/lib/asset-utils'

import { VideoStudioPromptComposer } from '../prompt-composer'

const assets: TemporaryAsset[] = [
  {
    id: 'asset-image-new',
    asset_type: 'image',
    name: '新角色立绘',
    preview_url: 'https://cdn.example/new.png',
    status: 'ACTIVE',
    created_at: 30,
    expires_at: 9999999999,
    verified_at: 31,
  },
  {
    id: 'asset-image-old',
    asset_type: 'image',
    name: '旧角色立绘',
    preview_url: 'https://cdn.example/old.png',
    status: 'ACTIVE',
    created_at: 10,
    expires_at: 9999999999,
    verified_at: 11,
  },
  {
    id: 'asset-video-failed',
    asset_type: 'video',
    name: '失败的视频',
    preview_url: 'https://cdn.example/failed.mp4',
    status: 'FAILED',
    created_at: 20,
    expires_at: 9999999999,
    verified_at: 21,
    error: { message: '审核未通过' },
  },
]

test('opens the categorized asset picker on @ and inserts one asset reference', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  const onMediaChange = vi.fn()
  function Fixture() {
    const [value, setValue] = useState('')
    return (
      <VideoStudioPromptComposer
        value={value}
        mode='references'
        media={[]}
        assets={assets}
        onChange={(next) => {
          onChange(next)
          setValue(next)
        }}
        onMediaChange={onMediaChange}
      />
    )
  }
  render(<Fixture />)

  await user.type(screen.getByRole('textbox', { name: 'Prompt' }), '@')

  expect(
    screen.getByRole('listbox', { name: 'Temporary assets' })
  ).toBeInTheDocument()
  expect(screen.getByText('新角色立绘')).toBeInTheDocument()
  expect(screen.getByText('旧角色立绘')).toBeInTheDocument()
  expect(
    screen.getAllByRole('option').map((option) => option.textContent)
  ).toEqual(expect.arrayContaining(['新角色立绘asset-image-new']))

  await user.click(screen.getByRole('option', { name: /新角色立绘/ }))

  expect(onChange).toHaveBeenLastCalledWith('@图片1 ')
  expect(onMediaChange).toHaveBeenCalledWith([
    expect.objectContaining({
      type: 'image',
      source: 'asset',
      value: 'asset-image-new',
      name: '新角色立绘',
      role: 'reference_image',
    }),
  ])
})

test('filters by media type and fuzzy searches asset name or id', async () => {
  const user = userEvent.setup()
  render(
    <VideoStudioPromptComposer
      value='@'
      mode='references'
      media={[]}
      assets={assets}
      onChange={vi.fn()}
      onMediaChange={vi.fn()}
    />
  )

  await user.click(screen.getByRole('button', { name: 'Video' }))
  expect(screen.getByText('失败的视频')).toBeInTheDocument()
  expect(screen.queryByText('新角色立绘')).not.toBeInTheDocument()

  await user.click(screen.getByRole('button', { name: 'Image' }))
  await user.type(screen.getByRole('searchbox'), 'asset-image-old')
  expect(screen.getByText('旧角色立绘')).toBeInTheDocument()
  expect(screen.queryByText('新角色立绘')).not.toBeInTheDocument()
})

test('renders selected assets inside the editor with availability state', () => {
  render(
    <VideoStudioPromptComposer
      value='@图片1 @视频1'
      mode='references'
      media={[
        {
          clientId: 'image',
          type: 'image',
          source: 'asset',
          role: 'reference_image',
          value: 'asset-image-new',
          name: '新角色立绘',
        },
        {
          clientId: 'video',
          type: 'video',
          source: 'asset',
          role: 'reference_video',
          value: 'asset-video-failed',
          name: '失败的视频',
        },
      ]}
      assets={assets}
      onChange={vi.fn()}
      onMediaChange={vi.fn()}
    />
  )

  expect(screen.getByTestId('prompt-asset-asset-image-new')).toHaveAttribute(
    'data-asset-status',
    'available'
  )
  expect(screen.getByTestId('prompt-asset-asset-video-failed')).toHaveAttribute(
    'data-asset-status',
    'unavailable'
  )
  expect(screen.getByText('审核未通过')).toBeInTheDocument()
})
