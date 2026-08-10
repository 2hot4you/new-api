const stages = [
  {
    title: '1. 提交',
    detail: '只发送一次付费 POST；保存返回的任务 ID。',
    tone: '#2563eb',
  },
  {
    title: '2. 轮询',
    detail: '用安全的 GET 查询状态，并对 429/5xx 退避。',
    tone: '#7c3aed',
  },
  {
    title: '3. 结算',
    detail: '读取最终结果和计费状态；refund_pending 表示退款仍在处理。',
    tone: '#059669',
  },
];

export default function ApiLifecycle() {
  return (
    <ol
      aria-label='异步 API 生命周期'
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: '1rem',
        listStyle: 'none',
        margin: '1.5rem 0',
        padding: 0,
      }}
    >
      {stages.map((stage) => (
        <li
          key={stage.title}
          style={{
            border: `1px solid ${stage.tone}`,
            borderLeftWidth: '0.35rem',
            borderRadius: '0.5rem',
            padding: '1rem',
          }}
        >
          <strong style={{ color: stage.tone }}>{stage.title}</strong>
          <div style={{ marginTop: '0.45rem' }}>{stage.detail}</div>
        </li>
      ))}
    </ol>
  );
}
