export default function Dashboard() {
  return (
    <div>
      <h1 className="text-2xl font-bold mb-4">系统总览</h1>
      <div className="grid grid-cols-2 gap-4">
        <div className="p-4 bg-white shadow rounded">
          <h2 className="text-lg font-semibold mb-2">Controller 节点</h2>
          <p>状态：🟢 正常</p>
          <p>Workload：12 个任务</p>
        </div>
        <div className="p-4 bg-white shadow rounded">
          <h2 className="text-lg font-semibold mb-2">Agent 节点</h2>
          <p>状态：🟢 正常</p>
          <p>Workload：23 个任务</p>
        </div>
      </div>
    </div>
  );
}