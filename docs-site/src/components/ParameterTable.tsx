import type { ReactNode } from 'react';

export interface ParameterRow {
  name: string;
  type: string;
  required: boolean | '条件';
  defaultValue?: string;
  description: ReactNode;
}

export interface ParameterTableProps {
  rows: ParameterRow[];
}

function requiredLabel(required: ParameterRow['required']): string {
  if (required === '条件') return required;
  return required ? '是' : '否';
}

export default function ParameterTable({ rows }: ParameterTableProps) {
  return (
    <table>
      <thead>
        <tr>
          <th scope='col'>参数</th>
          <th scope='col'>类型</th>
          <th scope='col'>必填</th>
          <th scope='col'>默认值</th>
          <th scope='col'>说明</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.name}>
            <th scope='row'><code>{row.name}</code></th>
            <td><code>{row.type}</code></td>
            <td>{requiredLabel(row.required)}</td>
            <td>{row.defaultValue ? <code>{row.defaultValue}</code> : '—'}</td>
            <td>{row.description}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
