/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'

import type { ModelRow } from '../model-pricing-snapshots'
import { buildModelRatioColumns } from '../model-ratio-table-columns'

const t = (key: string) => key

function ModelNameCell(props: { billingExpr: string; taskModel?: boolean }) {
  const model: ModelRow = {
    name: 'test-model',
    billingMode: 'tiered_expr',
    billingExpr: props.billingExpr,
    hasConflict: false,
    isDraftChanged: false,
    isDraftDeleted: false,
    isDraftNew: false,
  }
  const columns = buildModelRatioColumns({
    onDelete: vi.fn(),
    onEdit: vi.fn(),
    taskModelNames: props.taskModel ? new Set([model.name]) : new Set(),
    t,
  })
  const table = useReactTable({
    data: [model],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getAllCells()
    .find((item) => item.column.id === 'name')
  if (!cell) throw new Error('The model table must have a name column')
  return flexRender(cell.column.columnDef.cell, cell.getContext())
}

it('labels one unconditional tier as an expression', () => {
  render(<ModelNameCell billingExpr='tier("base", p * 3 + c * 15)' />)

  expect(screen.getByText('Expression')).toBeVisible()
  expect(screen.queryByText('Tiered')).not.toBeInTheDocument()
})

it('labels conditional branches as tiered', () => {
  render(
    <ModelNameCell billingExpr='len <= 200000 ? tier("base", p * 3 + c * 15) : tier("long", p * 6 + c * 22.5)' />
  )

  expect(screen.getByText('Tiered')).toBeVisible()
  expect(screen.queryByText('Expression')).not.toBeInTheDocument()
})

it('keeps the task-pricing badge for task models', () => {
  render(
    <ModelNameCell taskModel billingExpr='tier("base", u("seconds") * 0.4)' />
  )

  expect(screen.getByText('Task pricing')).toBeVisible()
  expect(screen.queryByText('Expression')).not.toBeInTheDocument()
  expect(screen.queryByText('Tiered')).not.toBeInTheDocument()
})
