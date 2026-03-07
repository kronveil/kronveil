import clsx from 'clsx';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';

interface MetricCardProps {
  title: string;
  value: string;
  change?: number;
  unit?: string;
  icon?: React.ReactNode;
}

export default function MetricCard({ title, value, change, unit, icon }: MetricCardProps) {
  const trend = change === undefined ? 'neutral' : change > 0 ? 'up' : change < 0 ? 'down' : 'neutral';

  return (
    <div className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-5 hover:border-[rgba(99,179,237,0.25)] transition-colors">
      <div className="flex items-start justify-between mb-3">
        <span className="text-xs text-[#718096] uppercase tracking-wider font-mono">
          {title}
        </span>
        {icon && (
          <div className="w-8 h-8 rounded-lg bg-[rgba(99,179,237,0.08)] flex items-center justify-center text-[#63b3ed]">
            {icon}
          </div>
        )}
      </div>
      <div className="flex items-baseline gap-2">
        <span className="text-3xl font-display font-extrabold bg-gradient-to-r from-[#63b3ed] to-[#4fd1c5] bg-clip-text text-transparent">
          {value}
        </span>
        {unit && <span className="text-sm text-[#718096]">{unit}</span>}
      </div>
      {change !== undefined && (
        <div className={clsx('flex items-center gap-1 mt-2 text-xs', {
          'text-[#68d391]': trend === 'up',
          'text-[#fc8181]': trend === 'down',
          'text-[#718096]': trend === 'neutral',
        })}>
          {trend === 'up' && <TrendingUp className="w-3 h-3" />}
          {trend === 'down' && <TrendingDown className="w-3 h-3" />}
          {trend === 'neutral' && <Minus className="w-3 h-3" />}
          {Math.abs(change)}% vs last hour
        </div>
      )}
    </div>
  );
}
