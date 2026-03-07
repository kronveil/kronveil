import { Activity, TrendingUp } from 'lucide-react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

const mockAnomalies = [
  { id: 'ANO-1', signal: 'kafka-consumer-lag', score: 0.94, source: 'kafka-collector', severity: 'critical', predicted: false, description: 'Consumer lag p99 spiked to 48s across payments-processor group', timestamp: new Date(Date.now() - 120_000).toISOString() },
  { id: 'ANO-2', signal: 'pod-memory-usage', score: 0.87, source: 'k8s-collector', severity: 'high', predicted: true, description: 'Memory usage trend predicts OOM within 15 minutes for downstream-payments', timestamp: new Date(Date.now() - 300_000).toISOString() },
  { id: 'ANO-3', signal: 'api-latency-p99', score: 0.72, source: 'k8s-collector', severity: 'medium', predicted: false, description: 'API gateway p99 latency increased 3x in last 10 minutes', timestamp: new Date(Date.now() - 600_000).toISOString() },
  { id: 'ANO-4', signal: 'node-cpu-utilization', score: 0.68, source: 'k8s-collector', severity: 'medium', predicted: true, description: 'Worker node CPU trending toward saturation at current growth rate', timestamp: new Date(Date.now() - 900_000).toISOString() },
  { id: 'ANO-5', signal: 'ci-build-duration', score: 0.55, source: 'cicd-collector', severity: 'low', predicted: false, description: 'Build times increased 40% after dependency update', timestamp: new Date(Date.now() - 1800_000).toISOString() },
];

const hourlyData = Array.from({ length: 24 }, (_, i) => ({
  hour: `${i}:00`,
  anomalies: Math.floor(Math.random() * 8),
  predicted: Math.floor(Math.random() * 3),
}));

const scoreColor = (score: number) => {
  if (score >= 0.9) return 'text-red-400';
  if (score >= 0.7) return 'text-orange-400';
  if (score >= 0.5) return 'text-yellow-400';
  return 'text-blue-400';
};

export default function Anomalies() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-display font-extrabold tracking-tight">Anomalies</h1>
        <p className="text-sm text-[#718096] mt-1">ML-powered anomaly detection and prediction</p>
      </div>

      <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-display font-bold">Anomaly Distribution (24h)</h2>
          <div className="flex items-center gap-4 text-[10px] font-mono">
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-[#63b3ed]" />Detected</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-[#4fd1c5]" />Predicted</span>
          </div>
        </div>
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={hourlyData}>
            <XAxis dataKey="hour" tick={{ fontSize: 10, fill: '#718096' }} axisLine={{ stroke: 'rgba(99,179,237,0.08)' }} tickLine={false} />
            <YAxis tick={{ fontSize: 10, fill: '#718096' }} axisLine={false} tickLine={false} />
            <Tooltip contentStyle={{ background: '#0c1526', border: '1px solid rgba(99,179,237,0.2)', borderRadius: '8px', fontSize: '12px' }} />
            <Bar dataKey="anomalies" fill="#63b3ed" radius={[4, 4, 0, 0]} />
            <Bar dataKey="predicted" fill="#4fd1c5" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>

      <div className="space-y-3">
        {mockAnomalies.map((anomaly) => (
          <div
            key={anomaly.id}
            className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5 hover:border-[rgba(99,179,237,0.25)] transition-colors"
          >
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center gap-3">
                <Activity className="w-4 h-4 text-[#63b3ed]" />
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{anomaly.signal}</span>
                    {anomaly.predicted && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono bg-[#4fd1c5]/10 text-[#4fd1c5] border border-[#4fd1c5]/20">
                        <TrendingUp className="w-2.5 h-2.5" />predicted
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-[#718096] mt-1">{anomaly.description}</p>
                </div>
              </div>
              <div className="text-right">
                <div className={`text-lg font-mono font-bold ${scoreColor(anomaly.score)}`}>
                  {(anomaly.score * 100).toFixed(0)}%
                </div>
                <div className="text-[10px] text-[#718096]">anomaly score</div>
              </div>
            </div>
            <div className="flex items-center gap-4 text-[10px] font-mono text-[#718096] mt-2">
              <span>{anomaly.source}</span>
              <span>{new Date(anomaly.timestamp).toLocaleTimeString()}</span>
              <span className="font-mono">{anomaly.id}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
