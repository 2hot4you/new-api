const stages = [
  {
    title: '1. 提交',
    detail: '只发送一次付费 POST；保存返回的任务 ID。',
  },
  {
    title: '2. 轮询',
    detail: '用安全的 GET 查询状态，并对 429/5xx 退避。',
  },
  {
    title: '3. 结算',
    detail: '读取最终结果和计费状态；refund_pending 表示退款仍在处理。',
  },
];

export default function ApiLifecycle() {
  return (
    <ol aria-label='异步 API 生命周期'>
      {stages.map((stage) => (
        <li key={stage.title}>
          <strong>{stage.title}</strong>
          <p>{stage.detail}</p>
        </li>
      ))}
    </ol>
  );
}
