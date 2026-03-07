import { Save } from 'lucide-react';

export default function Settings() {
  return (
    <div className="space-y-6 max-w-3xl">
      <div>
        <h1 className="text-2xl font-display font-extrabold tracking-tight">Settings</h1>
        <p className="text-sm text-[#718096] mt-1">Configure Kronveil agent behavior</p>
      </div>

      <div className="space-y-6">
        <section className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-6">
          <h2 className="text-sm font-display font-bold mb-4">AWS Bedrock</h2>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-[#718096] mb-1.5 font-mono">Region</label>
              <input type="text" defaultValue="us-east-1" className="w-full bg-[#040811] border border-[rgba(99,179,237,0.12)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-[#63b3ed] focus:outline-none" />
            </div>
            <div>
              <label className="block text-xs text-[#718096] mb-1.5 font-mono">Model ID</label>
              <input type="text" defaultValue="anthropic.claude-3-sonnet" className="w-full bg-[#040811] border border-[rgba(99,179,237,0.12)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-[#63b3ed] focus:outline-none" />
            </div>
          </div>
        </section>

        <section className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-6">
          <h2 className="text-sm font-display font-bold mb-4">Anomaly Detection</h2>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-[#718096] mb-1.5 font-mono">Sensitivity</label>
              <select className="w-full bg-[#040811] border border-[rgba(99,179,237,0.12)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-[#63b3ed] focus:outline-none">
                <option>Low</option>
                <option selected>Medium</option>
                <option>High</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-[#718096] mb-1.5 font-mono">Window Size</label>
              <input type="text" defaultValue="300" className="w-full bg-[#040811] border border-[rgba(99,179,237,0.12)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-[#63b3ed] focus:outline-none" />
            </div>
          </div>
        </section>

        <section className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-6">
          <h2 className="text-sm font-display font-bold mb-4">Remediation</h2>
          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" defaultChecked className="rounded border-[rgba(99,179,237,0.3)] bg-[#040811] text-[#63b3ed] focus:ring-[#63b3ed]" />
              <span className="text-sm text-gray-200">Enable auto-remediation</span>
            </label>
            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" defaultChecked className="rounded border-[rgba(99,179,237,0.3)] bg-[#040811] text-[#63b3ed] focus:ring-[#63b3ed]" />
              <span className="text-sm text-gray-200">Require approval for destructive actions</span>
            </label>
            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" className="rounded border-[rgba(99,179,237,0.3)] bg-[#040811] text-[#63b3ed] focus:ring-[#63b3ed]" />
              <span className="text-sm text-gray-200">Dry-run mode (simulate only)</span>
            </label>
          </div>
        </section>

        <section className="bg-[#070e1a] border border-[rgba(99,179,237,0.12)] rounded-xl p-6">
          <h2 className="text-sm font-display font-bold mb-4">Notifications</h2>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-[#718096] mb-1.5 font-mono">Slack Channel</label>
              <input type="text" defaultValue="#kronveil-alerts" className="w-full bg-[#040811] border border-[rgba(99,179,237,0.12)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-[#63b3ed] focus:outline-none" />
            </div>
            <div>
              <label className="block text-xs text-[#718096] mb-1.5 font-mono">PagerDuty Service</label>
              <input type="text" defaultValue="kronveil-prod" className="w-full bg-[#040811] border border-[rgba(99,179,237,0.12)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-[#63b3ed] focus:outline-none" />
            </div>
          </div>
        </section>

        <button className="flex items-center gap-2 bg-[#63b3ed] text-[#040811] px-5 py-2.5 rounded-lg font-semibold text-sm hover:bg-[#90cdf4] transition-colors">
          <Save className="w-4 h-4" />
          Save Configuration
        </button>
      </div>
    </div>
  );
}
