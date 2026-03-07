import { useState } from 'react';
import { AlertTriangle, Clock, ExternalLink } from 'lucide-react';
import StatusBadge from '../components/StatusBadge';

const mockIncidents = [
  {
    id: 'INC-2847',
    title: 'Pod OOM in downstream-payments',
    severity: 'critical' as const,
    status: 'resolved' as const,
    rootCause: 'Memory leak in batch processor after v2.3.1 deploy',
    affectedResources: ['downstream-payments-7b4f9', 'downstream-payments-8c5a1', 'downstream-payments-3d6e2'],
    createdAt: new Date(Date.now() - 3600_000).toISOString(),
    mttr: 23,
  },
  {
    id: 'INC-2846',
    title: 'Kafka consumer lag spike - payments-processor',
    severity: 'high' as const,
    status: 'resolved' as const,
    rootCause: 'Cascading failure from downstream-payments OOM',
    affectedResources: ['kafka-payments-topic', 'payments-processor-cg'],
    createdAt: new Date(Date.now() - 3500_000).toISOString(),
    mttr: 45,
  },
  {
    id: 'INC-2845',
    title: 'Node disk pressure on worker-node-12',
    severity: 'medium' as const,
    status: 'active' as const,
    rootCause: 'Log volume accumulation exceeding retention policy',
    affectedResources: ['worker-node-12', 'fluentbit-ds-x7k2p'],
    createdAt: new Date(Date.now() - 7200_000).toISOString(),
    mttr: undefined,
  },
  {
    id: 'INC-2844',
    title: 'Certificate expiry warning - api-gateway TLS',
    severity: 'low' as const,
    status: 'acknowledged' as const,
    rootCause: 'TLS certificate expires in 7 days, auto-renewal pending',
    affectedResources: ['api-gateway-tls-secret'],
    createdAt: new Date(Date.now() - 14400_000).toISOString(),
    mttr: undefined,
  },
];

const severityColor = {
  critical: 'text-red-400',
  high: 'text-orange-400',
  medium: 'text-yellow-400',
  low: 'text-blue-400',
};

export default function Incidents() {
  const [filter, setFilter] = useState<string>('all');
  const filtered = filter === 'all' ? mockIncidents : mockIncidents.filter((i) => i.status === filter);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-extrabold tracking-tight">Incidents</h1>
          <p className="text-sm text-[#718096] mt-1">AI-detected and auto-remediated incidents</p>
        </div>
        <div className="flex gap-2">
          {['all', 'active', 'acknowledged', 'resolved'].map((s) => (
            <button
              key={s}
              onClick={() => setFilter(s)}
              className={`px-3 py-1.5 rounded-lg text-xs font-mono transition-colors ${
                filter === s
                  ? 'bg-[rgba(99,179,237,0.15)] text-[#63b3ed] border border-[rgba(99,179,237,0.3)]'
                  : 'text-[#718096] hover:text-gray-200 border border-transparent'
              }`}
            >
              {s}
            </button>
          ))}
        </div>
      </div>

      <div className="space-y-3">
        {filtered.map((incident) => (
          <div
            key={incident.id}
            className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5 hover:border-[rgba(99,179,237,0.25)] transition-colors cursor-pointer"
          >
            <div className="flex items-start justify-between mb-3">
              <div className="flex items-center gap-3">
                <AlertTriangle className={`w-4 h-4 ${severityColor[incident.severity]}`} />
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-[#718096]">{incident.id}</span>
                    <span className="text-sm font-medium">{incident.title}</span>
                  </div>
                  <p className="text-xs text-[#718096] mt-1">{incident.rootCause}</p>
                </div>
              </div>
              <StatusBadge status={incident.status} pulse={incident.status === 'active'} />
            </div>

            <div className="flex items-center gap-4 text-[10px] font-mono text-[#718096]">
              <span className="flex items-center gap-1">
                <Clock className="w-3 h-3" />
                {new Date(incident.createdAt).toLocaleTimeString()}
              </span>
              {incident.mttr && (
                <span className="text-[#68d391]">MTTR: {incident.mttr}s</span>
              )}
              <span>{incident.affectedResources.length} resources affected</span>
              <ExternalLink className="w-3 h-3 ml-auto" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
