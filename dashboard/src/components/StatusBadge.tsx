import clsx from 'clsx';

type Status = 'healthy' | 'degraded' | 'critical' | 'unknown' | 'resolved' | 'active' | 'acknowledged';

const statusStyles: Record<Status, string> = {
  healthy: 'bg-green-500/10 text-green-400 border-green-500/20',
  degraded: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
  critical: 'bg-red-500/10 text-red-400 border-red-500/20',
  unknown: 'bg-gray-500/10 text-gray-400 border-gray-500/20',
  resolved: 'bg-green-500/10 text-green-400 border-green-500/20',
  active: 'bg-red-500/10 text-red-400 border-red-500/20',
  acknowledged: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
};

interface StatusBadgeProps {
  status: Status;
  pulse?: boolean;
}

export default function StatusBadge({ status, pulse }: StatusBadgeProps) {
  return (
    <span className={clsx(
      'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-mono border',
      statusStyles[status],
    )}>
      <span className={clsx('w-1.5 h-1.5 rounded-full', {
        'bg-green-400': status === 'healthy' || status === 'resolved',
        'bg-yellow-400': status === 'degraded' || status === 'acknowledged',
        'bg-red-400': status === 'critical' || status === 'active',
        'bg-gray-400': status === 'unknown',
        'animate-pulse': pulse,
      })} />
      {status}
    </span>
  );
}
