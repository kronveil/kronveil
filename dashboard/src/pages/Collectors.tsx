import { Radio, Cpu, Server, GitBranch, FileText } from 'lucide-react';
import StatusBadge from '../components/StatusBadge';

const mockCollectors = [
  { name: 'kubernetes', type: 'Kubernetes', icon: Server, status: 'healthy' as const, eventsPerSec: 4_200_000, targets: 312, errors: 0, lastEvent: '2s ago', details: '3 clusters, 54 nodes, 312 pods' },
  { name: 'kafka', type: 'Apache Kafka', icon: Radio, status: 'healthy' as const, eventsPerSec: 3_800_000, targets: 47, errors: 0, lastEvent: '1s ago', details: '5 clusters, 47 topics, 128 partitions' },
  { name: 'cloud-aws', type: 'AWS CloudWatch', icon: Cpu, status: 'healthy' as const, eventsPerSec: 1_500_000, targets: 89, errors: 2, lastEvent: '5s ago', details: '3 regions, 89 resources monitored' },
  { name: 'cicd', type: 'GitHub Actions', icon: GitBranch, status: 'degraded' as const, eventsPerSec: 12_000, targets: 15, errors: 3, lastEvent: '12s ago', details: '15 repositories, webhook-based collection' },
  { name: 'logs', type: 'Log Aggregator', icon: FileText, status: 'healthy' as const, eventsPerSec: 680_000, targets: 24, errors: 0, lastEvent: '1s ago', details: '24 log sources, JSON + logfmt parsers' },
];

export default function Collectors() {
  const totalEps = mockCollectors.reduce((sum, c) => sum + c.eventsPerSec, 0);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-display font-extrabold tracking-tight">Collectors</h1>
        <p className="text-sm text-[#718096] mt-1">Telemetry collection agents across your infrastructure</p>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Total Throughput</div>
          <div className="text-3xl font-display font-extrabold bg-gradient-to-r from-[#63b3ed] to-[#4fd1c5] bg-clip-text text-transparent">
            {(totalEps / 1_000_000).toFixed(1)}M
          </div>
          <div className="text-xs text-[#718096] mt-1">events/sec</div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Active Collectors</div>
          <div className="text-3xl font-display font-extrabold bg-gradient-to-r from-[#63b3ed] to-[#4fd1c5] bg-clip-text text-transparent">
            {mockCollectors.length}
          </div>
          <div className="text-xs text-[#718096] mt-1">across {mockCollectors.reduce((s, c) => s + c.targets, 0)} targets</div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Error Rate</div>
          <div className="text-3xl font-display font-extrabold text-[#68d391]">0.001%</div>
          <div className="text-xs text-[#718096] mt-1">{mockCollectors.reduce((s, c) => s + c.errors, 0)} errors total</div>
        </div>
      </div>

      <div className="space-y-3">
        {mockCollectors.map((collector) => {
          const Icon = collector.icon;
          return (
            <div
              key={collector.name}
              className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5 hover:border-[rgba(99,179,237,0.25)] transition-colors"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg bg-[rgba(99,179,237,0.08)] border border-[rgba(99,179,237,0.15)] flex items-center justify-center">
                    <Icon className="w-5 h-5 text-[#63b3ed]" />
                  </div>
                  <div>
                    <div className="text-sm font-medium">{collector.type}</div>
                    <div className="text-xs text-[#718096] font-mono">{collector.name}</div>
                  </div>
                </div>
                <StatusBadge status={collector.status} pulse={collector.status !== 'healthy'} />
              </div>
              <div className="grid grid-cols-4 gap-4 text-center">
                {[
                  { label: 'Events/sec', value: collector.eventsPerSec >= 1_000_000 ? `${(collector.eventsPerSec / 1_000_000).toFixed(1)}M` : `${(collector.eventsPerSec / 1_000).toFixed(0)}K` },
                  { label: 'Targets', value: collector.targets },
                  { label: 'Errors', value: collector.errors },
                  { label: 'Last Event', value: collector.lastEvent },
                ].map((metric) => (
                  <div key={metric.label}>
                    <div className="text-sm font-mono font-bold text-gray-200">{metric.value}</div>
                    <div className="text-[10px] text-[#718096] uppercase tracking-wider">{metric.label}</div>
                  </div>
                ))}
              </div>
              <div className="mt-3 pt-3 border-t border-[rgba(99,179,237,0.06)] text-xs text-[#718096]">
                {collector.details}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
