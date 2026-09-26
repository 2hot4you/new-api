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

import { VideoStudioPaidRequestDialog } from '../paid-request-dialog'

test('shows the estimated charge and only submits after explicit confirmation', async () => {
  const user = userEvent.setup()
  const onConfirm = vi.fn()
  const onOpenChange = vi.fn()
  render(
    <VideoStudioPaidRequestDialog
      open
      estimatedCost='¥15.5569'
      estimatedTokens={288625}
      pending={false}
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
    />
  )

  expect(screen.getByText('¥15.5569')).toBeInTheDocument()
  expect(screen.getByText('288,625')).toBeInTheDocument()
  expect(onConfirm).not.toHaveBeenCalled()

  await user.click(screen.getByRole('button', { name: 'Confirm and pay' }))
  expect(onConfirm).toHaveBeenCalledTimes(1)
})

test('labels smart-duration token estimates as an upper bound', () => {
  render(
    <VideoStudioPaidRequestDialog
      open
      estimatedCost='¥32.6472'
      estimatedTokens={605700}
      upperBound
      pending={false}
      onConfirm={vi.fn()}
      onOpenChange={vi.fn()}
    />
  )

  expect(screen.getByText('Estimated maximum cost')).toBeInTheDocument()
  expect(
    screen.getByText(/Automatic duration is estimated using the model maximum/)
  ).toBeInTheDocument()
})
