/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { VideoStudioMediaPicker } from '../media-picker'

test('adds a public image URL as the first frame', async () => {
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

  await user.type(
    screen.getByRole('textbox', { name: 'Public media URL' }),
    'https://cdn.example.com/first-frame.png'
  )
  await user.click(screen.getByRole('button', { name: 'Add URL' }))

  expect(onChange).toHaveBeenCalledWith([
    expect.objectContaining({
      type: 'image',
      source: 'url',
      role: 'first_frame',
      value: 'https://cdn.example.com/first-frame.png',
    }),
  ])
  expect(screen.getByText('First frame · Required')).toBeInTheDocument()
  expect(screen.getByText('Last frame · Optional')).toBeInTheDocument()
})

test('prevents adding a third image to first and last frame mode', () => {
  render(
    <VideoStudioMediaPicker
      mode='frames'
      media={[
        {
          clientId: 'first',
          type: 'image',
          source: 'url',
          role: 'first_frame',
          value: 'https://cdn.example.com/first.png',
          name: 'first.png',
        },
        {
          clientId: 'last',
          type: 'image',
          source: 'url',
          role: 'last_frame',
          value: 'https://cdn.example.com/last.png',
          name: 'last.png',
        },
      ]}
      assets={[]}
      onChange={vi.fn()}
      onAssetsChanged={vi.fn()}
    />
  )

  expect(screen.getByRole('button', { name: 'Add URL' })).toBeDisabled()
  expect(
    screen.getByRole('button', { name: 'Upload from device' })
  ).toBeDisabled()
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
    screen.queryByRole('textbox', { name: 'Public media URL' })
  ).not.toBeInTheDocument()
})
