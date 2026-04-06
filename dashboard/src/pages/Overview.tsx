import { Activity, AlertTriangle, Zap, Server, Clock, Shield, Radio } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import MetricCard from '../components/MetricCard';
import StatusBadge from '../components/StatusBadge';
import EventTimeline from '../components/EventTimeline';
import { useEventStream } from '../hooks/useEventStream';

const throughputData = Array.from({ length: 24 }, (_, i) => ({
  time: `${i}:00`,
  events: Math.floor(8_000_000 + Math.random() * 4_000_000),
  anomalies: Math.floor(Math.random() * 15),
}));

const recentEvents = [
  {
    id: '1',
    timestamp: new Date(Date.now() - 120_000).toISOString(),
    type: 'anomaly' as const,
    title: 'Kafka consumer lag spike detected',
    description: 'Consumer group payments-processor lag increased to 48s (p99) across 3 partitions',
  },
  {
    id: '2',
    timestamp: new Date(Date.now() - 95_000).toISOString(),
    type: 'incident' as const,
    title: 'INC-2847: Pod OOM in downstream-payments',
    description: 'Root cause identified: memory leak in batch processor after v2.3.1 deploy',
  },
  {
    id: '3',
    timestamp: new Date(Date.now() - 70_000).toISOString(),
    type: 'remediation' as const,
    title: 'Auto-remediated: Scaled deployment',
    description: 'Scaled downstream-payments from 5 to 8 replicas. Lag normalized within 23s.',
  },
  {
    id: '4',
    timestamp: new Date(Date.now() - 30_000).toISOString(),
    type: 'info' as const,
    title: 'Capacity forecast updated',
    description: 'Projected 15% increase in compute needed for payments cluster by next quarter',
  },
];

const clusterHealth = [
  { name: 'prod-us-east-1', status: 'healthy' as const, nodes: 24, pods: 312 },
  { name: 'prod-eu-west-1', status: 'healthy' as const, nodes: 18, pods: 247 },
  { name: 'prod-ap-south-1', status: 'degraded' as const, nodes: 12, pods: 156 },
];

export default function Overview() {
  const { events: liveEvents, connected: wsConnected } = useEventStream();

  // Use live WebSocket events when connected and available, otherwise fall back to mock data
  const displayEvents = wsConnected && liveEvents.length > 0 ? liveEvents : recentEvents;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-display font-extrabold tracking-tight">Overview</h1>
        {wsConnected && (
          <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-green-500/10 text-green-400 border border-green-500/20">
            <Radio className="w-3 h-3 animate-pulse" />
            Live
          </span>
        )}
      </div>
      <div>
        <p className="text-sm text-[#718096] mt-1">Real-time infrastructure intelligence</p>
      </div>

      <div className="grid grid-cols-4 gap-4">
        <MetricCard
          title="Events / sec"
          value="10.2M"
          change={3.2}
          icon={<Zap className="w-4 h-4" />}
        />
        <MetricCard
          title="Active Incidents"
          value="2"
          change={-50}
          icon={<AlertTriangle className="w-4 h-4" />}
        />
        <MetricCard
          title="Avg MTTR"
          value="23"
          unit="sec"
          change={-62}
          icon={<Clock className="w-4 h-4" />}
        />
        <MetricCard
          title="Anomalies (24h)"
          value="47"
          change={12}
          icon={<Activity className="w-4 h-4" />}
        />
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="col-span-2 bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-display font-bold tracking-tight">Event Throughput</h2>
            <span className="text-[10px] font-mono text-[#718096]">Last 24 hours</span>
          </div>
          <ResponsiveContainer width="100%" height={240}>
            <AreaChart data={throughputData}>
              <defs>
                <linearGradient id="eventGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#63b3ed" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#63b3ed" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="time"
                tick={{ fontSize: 10, fill: '#718096' }}
                axisLine={{ stroke: 'rgba(99,179,237,0.08)' }}
                tickLine={false}
              />
              <YAxis
                tick={{ fontSize: 10, fill: '#718096' }}
                axisLine={false}
                tickLine={false}
                tickFormatter={(v: number) => `${(v / 1_000_000).toFixed(0)}M`}
              />
              <Tooltip
                contentStyle={{
                  background: '#0c1526',
                  border: '1px solid rgba(99,179,237,0.2)',
                  borderRadius: '8px',
                  fontSize: '12px',
                }}
              />
              <Area
                type="monotone"
                dataKey="events"
                stroke="#63b3ed"
                fill="url(#eventGrad)"
                strokeWidth={2}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <h2 className="text-sm font-display font-bold tracking-tight mb-4">Cluster Health</h2>
          <div className="space-y-3">
            {clusterHealth.map((cluster) => (
              <div
                key={cluster.name}
                className="flex items-center justify-between p-3 rounded-lg bg-[rgba(255,255,255,0.02)] border border-[rgba(99,179,237,0.06)]"
              >
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <Server className="w-3 h-3 text-[#718096]" />
                    <span className="text-xs font-medium">{cluster.name}</span>
                  </div>
                  <span className="text-[10px] text-[#718096] font-mono">
                    {cluster.nodes} nodes &middot; {cluster.pods} pods
                  </span>
                </div>
                <StatusBadge status={cluster.status} pulse={cluster.status !== 'healthy'} />
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="col-span-2 bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-display font-bold tracking-tight">Live Event Feed</h2>
            {wsConnected ? (
              <span className="text-[10px] font-mono text-green-400">streaming</span>
            ) : (
              <span className="text-[10px] font-mono text-[#718096]">polling</span>
            )}
          </div>
          <EventTimeline events={displayEvents} />
        </div>

        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <h2 className="text-sm font-display font-bold tracking-tight mb-4">Policy Compliance</h2>
          <div className="space-y-3">
            {[
              { name: 'Resource Limits', compliant: 98 },
              { name: 'Network Policy', compliant: 100 },
              { name: 'Image Scanning', compliant: 95 },
              { name: 'RBAC', compliant: 100 },
              { name: 'Secret Rotation', compliant: 87 },
            ].map((policy) => (
              <div key={policy.name}>
                <div className="flex justify-between text-xs mb-1">
                  <span className="text-[#718096]">{policy.name}</span>
                  <span className={policy.compliant < 90 ? 'text-[#f6ad55]' : 'text-[#68d391]'}>
                    {policy.compliant}%
                  </span>
                </div>
                <div className="h-1.5 bg-[rgba(99,179,237,0.08)] rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full ${policy.compliant < 90 ? 'bg-[#f6ad55]' : 'bg-[#4fd1c5]'}`}
                    style={{ width: `${policy.compliant}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
          <div className="mt-4 pt-3 border-t border-[rgba(99,179,237,0.08)]">
            <div className="flex items-center gap-2 text-xs text-[#718096]">
              <Shield className="w-3 h-3" />
              <span>5 active OPA policies</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
