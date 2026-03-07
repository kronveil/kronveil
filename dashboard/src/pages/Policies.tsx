import { Shield, Check, X, AlertTriangle } from 'lucide-react';

const mockPolicies = [
  { id: 'pol-1', name: 'require-resource-limits', description: 'All pods must define CPU and memory resource limits', severity: 'high', enabled: true, violations: 7, lastEvaluated: '30s ago', category: 'Kubernetes' },
  { id: 'pol-2', name: 'enforce-network-policy', description: 'Every namespace must have a default deny NetworkPolicy', severity: 'critical', enabled: true, violations: 0, lastEvaluated: '30s ago', category: 'Security' },
  { id: 'pol-3', name: 'image-scan-required', description: 'Container images must pass vulnerability scanning before deployment', severity: 'critical', enabled: true, violations: 3, lastEvaluated: '1m ago', category: 'Security' },
  { id: 'pol-4', name: 'rbac-least-privilege', description: 'Service accounts must not use cluster-admin role', severity: 'critical', enabled: true, violations: 0, lastEvaluated: '30s ago', category: 'Security' },
  { id: 'pol-5', name: 'secret-rotation-30d', description: 'All secrets must be rotated within 30-day window', severity: 'medium', enabled: true, violations: 12, lastEvaluated: '2m ago', category: 'Compliance' },
  { id: 'pol-6', name: 'kafka-min-isr', description: 'Kafka topics must maintain min.insync.replicas >= 2', severity: 'high', enabled: true, violations: 0, lastEvaluated: '30s ago', category: 'Kafka' },
  { id: 'pol-7', name: 'no-latest-tag', description: 'Container images must not use :latest tag', severity: 'medium', enabled: false, violations: 0, lastEvaluated: 'disabled', category: 'Kubernetes' },
];

export default function Policies() {
  const activeCount = mockPolicies.filter((p) => p.enabled).length;
  const totalViolations = mockPolicies.reduce((s, p) => s + p.violations, 0);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-display font-extrabold tracking-tight">Policy Engine</h1>
        <p className="text-sm text-[#718096] mt-1">OPA-based governance and compliance enforcement</p>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Active Policies</div>
          <div className="text-3xl font-display font-extrabold bg-gradient-to-r from-[#63b3ed] to-[#4fd1c5] bg-clip-text text-transparent">{activeCount}</div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Total Violations</div>
          <div className="text-3xl font-display font-extrabold text-[#f6ad55]">{totalViolations}</div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Compliance Rate</div>
          <div className="text-3xl font-display font-extrabold text-[#68d391]">96.8%</div>
        </div>
      </div>

      <div className="space-y-3">
        {mockPolicies.map((policy) => (
          <div
            key={policy.id}
            className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5 hover:border-[rgba(99,179,237,0.25)] transition-colors"
          >
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <Shield className={`w-4 h-4 ${policy.enabled ? 'text-[#63b3ed]' : 'text-[#718096]'}`} />
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-mono font-medium">{policy.name}</span>
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-[rgba(99,179,237,0.08)] text-[#718096]">
                      {policy.category}
                    </span>
                  </div>
                  <p className="text-xs text-[#718096] mt-1">{policy.description}</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                {policy.violations > 0 ? (
                  <span className="flex items-center gap-1 text-xs text-[#f6ad55]">
                    <AlertTriangle className="w-3 h-3" />{policy.violations} violations
                  </span>
                ) : policy.enabled ? (
                  <span className="flex items-center gap-1 text-xs text-[#68d391]">
                    <Check className="w-3 h-3" />compliant
                  </span>
                ) : (
                  <span className="flex items-center gap-1 text-xs text-[#718096]">
                    <X className="w-3 h-3" />disabled
                  </span>
                )}
              </div>
            </div>
            <div className="flex items-center gap-4 mt-3 text-[10px] font-mono text-[#718096]">
              <span>severity: {policy.severity}</span>
              <span>evaluated: {policy.lastEvaluated}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
