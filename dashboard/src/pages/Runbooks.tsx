import { BookOpen, Play, CheckCircle, XCircle, Clock, Zap } from 'lucide-react';

const mockRunbooks = [
  {
    id: 'rb-1',
    name: 'Pod OOM Runbook',
    description: 'Automated remediation for out-of-memory pod crashes',
    incidentTypes: ['OOMKilled', 'MemoryPressure'],
    steps: 3,
    autoExecute: true,
    lastRun: new Date(Date.now() - 2 * 3600_000).toISOString(),
    successRate: 95,
    totalExecutions: 42,
    recentRuns: ['success', 'success', 'failure'] as const,
  },
  {
    id: 'rb-2',
    name: 'High Latency Runbook',
    description: 'Diagnose and mitigate high latency in upstream services',
    incidentTypes: ['HighLatency', 'SLOBreach'],
    steps: 3,
    autoExecute: false,
    lastRun: new Date(Date.now() - 45 * 60_000).toISOString(),
    successRate: 88,
    totalExecutions: 27,
    recentRuns: ['success', 'failure', 'success'] as const,
  },
  {
    id: 'rb-3',
    name: 'Disk Pressure Runbook',
    description: 'Clean up disk space and adjust log retention policies',
    incidentTypes: ['DiskPressure', 'LogVolumeHigh'],
    steps: 2,
    autoExecute: true,
    lastRun: new Date(Date.now() - 6 * 3600_000).toISOString(),
    successRate: 100,
    totalExecutions: 15,
    recentRuns: ['success', 'success', 'success'] as const,
  },
  {
    id: 'rb-4',
    name: 'Certificate Expiry Runbook',
    description: 'Renew and rotate TLS certificates before expiration',
    incidentTypes: ['CertExpiry', 'TLSError'],
    steps: 2,
    autoExecute: false,
    lastRun: null,
    successRate: 0,
    totalExecutions: 0,
    recentRuns: [] as const,
  },
];

function formatTimeAgo(isoString: string | null): string {
  if (!isoString) return 'Never';
  const diff = Date.now() - new Date(isoString).getTime();
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export default function Runbooks() {
  const totalRunbooks = mockRunbooks.length;
  const autoExecuteCount = mockRunbooks.filter((r) => r.autoExecute).length;
  const totalExecutions24h = mockRunbooks.reduce((s, r) => s + r.totalExecutions, 0);
  const avgSuccessRate =
    mockRunbooks.filter((r) => r.totalExecutions > 0).length > 0
      ? Math.round(
          mockRunbooks
            .filter((r) => r.totalExecutions > 0)
            .reduce((s, r) => s + r.successRate, 0) /
            mockRunbooks.filter((r) => r.totalExecutions > 0).length,
        )
      : 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-display font-extrabold tracking-tight">Runbooks</h1>
        <p className="text-sm text-[#718096] mt-1">Automated incident response playbooks</p>
      </div>

      <div className="grid grid-cols-4 gap-4">
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Total Runbooks</div>
          <div className="text-3xl font-display font-extrabold bg-gradient-to-r from-[#63b3ed] to-[#4fd1c5] bg-clip-text text-transparent">
            {totalRunbooks}
          </div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Auto-Execute</div>
          <div className="text-3xl font-display font-extrabold bg-gradient-to-r from-[#63b3ed] to-[#4fd1c5] bg-clip-text text-transparent">
            {autoExecuteCount}
          </div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Executions (24h)</div>
          <div className="text-3xl font-display font-extrabold text-[#f6ad55]">{totalExecutions24h}</div>
        </div>
        <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
          <div className="text-xs text-[#718096] uppercase tracking-wider font-mono mb-2">Success Rate</div>
          <div className="text-3xl font-display font-extrabold text-[#68d391]">{avgSuccessRate}%</div>
        </div>
      </div>

      <div className="space-y-3">
        {mockRunbooks.map((runbook) => (
          <div
            key={runbook.id}
            className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5 hover:border-[rgba(99,179,237,0.25)] transition-colors"
          >
            <div className="flex items-start justify-between mb-3">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-[rgba(99,179,237,0.08)] border border-[rgba(99,179,237,0.15)] flex items-center justify-center">
                  <BookOpen className="w-5 h-5 text-[#63b3ed]" />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{runbook.name}</span>
                    <span className="flex items-center gap-1">
                      {runbook.autoExecute ? (
                        <>
                          <span className="w-2 h-2 rounded-full bg-[#68d391]" />
                          <span className="text-[10px] font-mono text-[#68d391]">auto</span>
                        </>
                      ) : (
                        <>
                          <span className="w-2 h-2 rounded-full bg-[#718096]" />
                          <span className="text-[10px] font-mono text-[#718096]">manual</span>
                        </>
                      )}
                    </span>
                  </div>
                  <p className="text-xs text-[#718096] mt-1">{runbook.description}</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {runbook.recentRuns.map((result, i) => (
                  <span
                    key={i}
                    className={`w-2.5 h-2.5 rounded-full ${
                      result === 'success' ? 'bg-[#68d391]' : 'bg-[#fc8181]'
                    }`}
                    title={result}
                  />
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2 mb-3">
              {runbook.incidentTypes.map((type) => (
                <span
                  key={type}
                  className="px-2 py-0.5 rounded text-[10px] font-mono bg-[rgba(99,179,237,0.08)] text-[#718096]"
                >
                  {type}
                </span>
              ))}
            </div>

            <div className="flex items-center gap-4 text-[10px] font-mono text-[#718096]">
              <span className="flex items-center gap-1">
                <Play className="w-3 h-3" />
                {runbook.steps} steps
              </span>
              <span className="flex items-center gap-1">
                <Clock className="w-3 h-3" />
                {formatTimeAgo(runbook.lastRun)}
              </span>
              {runbook.totalExecutions > 0 && (
                <span className="flex items-center gap-1">
                  <CheckCircle className="w-3 h-3 text-[#68d391]" />
                  {runbook.successRate}% success
                </span>
              )}
              <span className="flex items-center gap-1">
                <Zap className="w-3 h-3" />
                {runbook.totalExecutions} executions
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
