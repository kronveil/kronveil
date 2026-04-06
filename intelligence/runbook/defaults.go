package runbook

import "time"

// DefaultRunbooks returns pre-built runbooks for common incident scenarios.
func DefaultRunbooks() []*Runbook {
	return []*Runbook{
		{
			ID:            "pod-oom-runbook",
			Name:          "Pod OOM Remediation",
			Description:   "Automatically scales up deployment and restarts OOM-killed pods, then notifies the on-call team.",
			IncidentTypes: []string{"pod_oom"},
			Steps: []Step{
				{
					Name:   "scale-up-deployment",
					Action: "kubectl_scale",
					Params: map[string]string{
						"deployment": "affected-deployment",
						"replicas":   "3",
						"namespace":  "default",
					},
					ContinueOnError: false,
					Timeout:         30 * time.Second,
				},
				{
					Name:   "restart-oom-pod",
					Action: "restart_pod",
					Params: map[string]string{
						"pod":       "affected-pod",
						"namespace": "default",
					},
					ContinueOnError: true,
					Timeout:         30 * time.Second,
				},
				{
					Name:   "notify-oncall-team",
					Action: "notify_oncall",
					Params: map[string]string{
						"channel": "#oncall",
						"message": "Pod OOM detected: deployment scaled up and pod restarted",
					},
					ContinueOnError: true,
					Timeout:         10 * time.Second,
				},
			},
			Enabled:     true,
			AutoExecute: false,
			MaxRetries:  2,
			Timeout:     2 * time.Minute,
		},
		{
			ID:            "high-latency-runbook",
			Name:          "High Latency Investigation",
			Description:   "Runs diagnostics to identify latency sources, checks dependency health, and notifies the on-call team.",
			IncidentTypes: []string{"high_latency"},
			Steps: []Step{
				{
					Name:   "run-latency-diagnostic",
					Action: "run_diagnostic",
					Params: map[string]string{
						"command": "kubectl top pods --sort-by=cpu -A",
					},
					ContinueOnError: true,
					Timeout:         30 * time.Second,
				},
				{
					Name:   "check-dependency-health",
					Action: "run_diagnostic",
					Params: map[string]string{
						"command": "kubectl get endpoints -A --no-headers | grep '<none>'",
					},
					ContinueOnError: true,
					Timeout:         30 * time.Second,
				},
				{
					Name:   "notify-oncall-team",
					Action: "notify_oncall",
					Params: map[string]string{
						"channel": "#oncall",
						"message": "High latency detected: diagnostics collected, review required",
					},
					ContinueOnError: true,
					Timeout:         10 * time.Second,
				},
			},
			Enabled:     true,
			AutoExecute: false,
			MaxRetries:  1,
			Timeout:     3 * time.Minute,
		},
		{
			ID:            "disk-pressure-runbook",
			Name:          "Disk Pressure Remediation",
			Description:   "Runs cleanup diagnostics to free disk space and notifies the on-call team.",
			IncidentTypes: []string{"disk_pressure"},
			Steps: []Step{
				{
					Name:   "run-disk-cleanup",
					Action: "run_diagnostic",
					Params: map[string]string{
						"command": "kubectl exec -it cleanup-agent -- /bin/sh -c 'df -h && find /tmp -mtime +7 -delete'",
					},
					ContinueOnError: true,
					Timeout:         60 * time.Second,
				},
				{
					Name:   "notify-oncall-team",
					Action: "notify_oncall",
					Params: map[string]string{
						"channel": "#oncall",
						"message": "Disk pressure detected: cleanup executed, verify disk usage",
					},
					ContinueOnError: true,
					Timeout:         10 * time.Second,
				},
			},
			Enabled:     true,
			AutoExecute: false,
			MaxRetries:  1,
			Timeout:     2 * time.Minute,
		},
		{
			ID:            "certificate-expiry-runbook",
			Name:          "Certificate Expiry Remediation",
			Description:   "Runs certificate renewal process and notifies the on-call team.",
			IncidentTypes: []string{"certificate_expiry"},
			Steps: []Step{
				{
					Name:   "renew-certificates",
					Action: "custom_script",
					Params: map[string]string{
						"script": "/opt/kronveil/scripts/renew-certs.sh",
					},
					ContinueOnError: false,
					Timeout:         120 * time.Second,
				},
				{
					Name:   "notify-oncall-team",
					Action: "notify_oncall",
					Params: map[string]string{
						"channel": "#oncall",
						"message": "Certificate expiry detected: renewal script executed, verify certificate status",
					},
					ContinueOnError: true,
					Timeout:         10 * time.Second,
				},
			},
			Enabled:     true,
			AutoExecute: false,
			MaxRetries:  2,
			Timeout:     5 * time.Minute,
		},
	}
}
